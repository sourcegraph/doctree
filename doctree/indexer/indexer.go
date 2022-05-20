package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/sourcegraph/doctree/doctree/apischema"
	"github.com/sourcegraph/doctree/doctree/schema"
)

// Language describes an indexer for a specific language.
type Language interface {
	// Name of the language this indexer works for.
	Name() schema.Language

	// Extensions returns a list of file extensions commonly associated with the language.
	Extensions() []string

	// IndexDir indexes a directory of code likely to contain sources in this language recursively.
	IndexDir(ctx context.Context, dir string) (*schema.Index, error)
}

// Registered indexers by language ID ("go", "objc", "cpp", etc.)
var Registered = map[string]Language{}

// Registers a doctree language indexer.
func Register(indexer Language) {
	Registered[indexer.Name().ID] = indexer
}

// IndexDir indexes the specified directory recursively. It looks at the file extension of every
// file, and then asks the registered indexers for each language to index.
//
// Returns the successful indexes and any errors.
func IndexDir(ctx context.Context, dir string) (map[string]*schema.Index, error) {
	// Identify all file extensions in the directory recursively.
	extensions := map[string]struct{}{}
	if err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // error walking dir
		}
		ext := filepath.Ext(path)
		if ext != "" && ext != "." {
			ext = ext[1:] // ".txt" -> "txt"
			extensions[ext] = struct{}{}
		}
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "WalkDir")
	}

	// Map extensions to indexers.
	indexersByExtension := map[string][]Language{}
	for _, language := range Registered {
		for _, ext := range language.Extensions() {
			indexers := indexersByExtension[ext]
			indexers = append(indexers, language)
			indexersByExtension[ext] = indexers
		}
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, errors.Wrap(err, "Abs")
	}

	// Run indexers for each language.
	var (
		wg sync.WaitGroup

		mu      sync.Mutex
		errs    error
		results = map[string]*schema.Index{}
	)
	// TODO: configurable parallelism?
	for ext := range extensions {
		ext := ext
		for _, indexer := range indexersByExtension[ext] {
			indexer := indexer
			wg.Add(1)
			go func() {
				defer wg.Done()
				start := time.Now()
				index, err := indexer.IndexDir(ctx, dir)
				if index != nil {
					index.DurationSeconds = time.Since(start).Seconds()
					index.CreatedAt = time.Now().Format(time.RFC3339)
					index.Directory = absDir
				}

				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errs = multierror.Append(errs, errors.Wrap(err, indexer.Name().ID+": IndexDir"))
				} else {
					results[indexer.Name().ID] = index
				}
			}()
		}
	}
	wg.Wait()
	return results, errs
}

// WriteIndexes writes indexes to the index data directory:
//
// index/<project_name>/<language_id>
func WriteIndexes(projectName string, indexDataDir string, indexes map[string]*schema.Index) error {
	// TODO: binary format?
	// TODO: compression

	// Ensure paths are absolute first. Index ID is absolute path of indexed directory effectively.
	var err error
	indexDataDir, err = filepath.Abs(indexDataDir)
	if err != nil {
		return errors.Wrap(err, "Abs")
	}

	outDir := filepath.Join(indexDataDir, encodeProjectName(projectName))

	// Delete any old index data in this dir (e.g. if we had python+go before, but now only go, we
	// need to delete python index.)
	if err := os.RemoveAll(outDir); err != nil {
		return errors.Wrap(err, "RemoveAll")
	}
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return errors.Wrap(err, "MkdirAll")
	}

	for lang, index := range indexes {
		f, err := os.Create(filepath.Join(outDir, lang))
		if err != nil {
			return errors.Wrap(err, "Create")
		}
		defer f.Close()

		if err := json.NewEncoder(f).Encode(index); err != nil {
			return errors.Wrap(err, "Encode")
		}
	}
	return nil
}

// Lists all indexes found in the index data directory.
func List(indexDataDir string) ([]string, error) {
	dir, err := ioutil.ReadDir(indexDataDir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "ReadDir")
	}
	var indexes []string
	for _, info := range dir {
		if info.IsDir() {
			indexes = append(indexes, decodeProjectName(info.Name()))
		}
	}
	return indexes, nil
}

var (
	jsonDecodeCacheMu sync.RWMutex
	jsonDecodeCache   = map[string]struct {
		modTime time.Time
		value   schema.Index
	}{}
)

// GetIndex gets all the language indexes for the specified project.
//
// When autoCloneMissing is true, if the project does not exist the server will attempt to
// `git clone <projectName> and index it. Beware, this may not be safe to enable if you have Git
// configured to access private repositories and the server is public!
func GetIndex(ctx context.Context, dataDir, indexDataDir, projectName string, autoCloneMissing bool) (apischema.ProjectIndexes, error) {
	indexName := encodeProjectName(projectName)
	if strings.Contains(indexName, "/") || strings.Contains(indexName, "..") {
		return nil, errors.New("potentially malicious index name (this is likely a bug)")
	}

	indexes := apischema.ProjectIndexes{}
	dir, err := ioutil.ReadDir(filepath.Join(indexDataDir, indexName))
	if os.IsNotExist(err) {
		if autoCloneMissing {
			repositoryURL := "https://" + projectName
			log.Println("cloning", repositoryURL)
			if err := cloneAndIndex(ctx, repositoryURL, dataDir); err != nil {
				log.Println("failed to clone", repositoryURL, "error:", err)
				return nil, errors.Wrap(err, "cloneAndIndex")
			}
			return GetIndex(ctx, dataDir, indexDataDir, projectName, false)
		}
	}
	if err != nil {
		return nil, errors.Wrap(err, "ReadDir")
	}
	for _, info := range dir {
		if !info.IsDir() && info.Name() != "search-index.sinter" && info.Name() != "version" {
			lang := info.Name()

			indexFile := filepath.Join(indexDataDir, indexName, lang)
			f, err := os.Open(indexFile)
			if err != nil {
				return nil, errors.Wrap(err, "Open")
			}
			defer f.Close()

			stat, err := f.Stat()
			if err != nil {
				return nil, errors.Wrap(err, "Stat")
			}

			jsonDecodeCacheMu.RLock()
			cached, ok := jsonDecodeCache[indexFile]
			jsonDecodeCacheMu.RUnlock()
			if ok && cached.modTime == stat.ModTime() {
				indexes[lang] = cached.value
				continue
			}

			var decoded schema.Index
			if err := json.NewDecoder(f).Decode(&decoded); err != nil {
				return nil, errors.Wrap(err, "Decode")
			}
			jsonDecodeCacheMu.Lock()
			jsonDecodeCache[indexFile] = struct {
				modTime time.Time
				value   schema.Index
			}{
				modTime: stat.ModTime(),
				value:   decoded,
			}
			jsonDecodeCacheMu.Unlock()

			indexes[lang] = decoded
		}
	}
	return indexes, nil
}

func cloneAndIndex(ctx context.Context, repositoryURL, dataDir string) error {
	// Clone the repository into a temp dir.
	dir, err := os.MkdirTemp(os.TempDir(), "doctree-clone")
	if err != nil {
		return errors.Wrap(err, "TempDir")
	}
	defer os.RemoveAll(dir)

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", repositoryURL, "repo/")
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %v\n%s", strings.Join(cmd.Args, " "), err, out)
	}

	// Index the repository.
	projectName := strings.TrimPrefix(repositoryURL, "https://")
	if err := RunIndexers(ctx, filepath.Join(dir, "repo"), dataDir, projectName); err != nil {
		return errors.Wrap(err, "RunIndexers")
	}
	return err
}

func encodeProjectName(name string) string {
	return strings.ReplaceAll(name, "/", "---")
}

func decodeProjectName(name string) string {
	return strings.ReplaceAll(name, "---", "/")
}

type AutoIndexedProject struct {
	// Name of the project to be auto-indexed
	Name string `json:"name"`
}

// Runs all the registered language indexes along with the search indexer and stores the results.
//
// If an error is returned, it may be the case that some indexers succeeded while others failed.
func RunIndexers(ctx context.Context, dir, dataDir, projectName string) error {
	var err error

	// Ensure the doctree data dir exists, and that it has a version file.
	if err := ensureDataDir(dataDir); err != nil {
		return errors.Wrap(err, "ensureDataDir")
	}

	// IndexDir may partially complete, with some indexers succeeding while others fail. In this
	// case indexes and indexErr are both != nil.
	indexes, indexErr := IndexDir(ctx, dir)
	for _, index := range indexes {
		fmt.Printf("%v: indexed %v files (%v bytes) in %v\n", index.Language.ID, index.NumFiles, index.NumBytes, time.Duration(index.DurationSeconds*float64(time.Second)).Round(time.Millisecond))
	}
	if indexErr != nil {
		err = multierror.Append(err, errors.Wrap(indexErr, "IndexDir"))
	}

	// Write indexes that we did produce.
	indexDataDir := filepath.Join(dataDir, "index")
	writeErr := WriteIndexes(projectName, indexDataDir, indexes)
	if writeErr != nil {
		err = multierror.Append(err, errors.Wrap(writeErr, "WriteIndexes"))
	}

	// Index for search the indexes that we did produce.
	projectDir := filepath.Join(indexDataDir, encodeProjectName(projectName))
	searchErr := IndexForSearch(projectName, indexDataDir, indexes)
	if searchErr != nil {
		if rmErr := os.RemoveAll(projectDir); rmErr != nil {
			err = multierror.Append(err, errors.Wrap(rmErr, "RemoveAll"))
		}
		err = multierror.Append(err, errors.Wrap(searchErr, "IndexForSearch"))
	}

	// Write a version number file.
	versionErr := os.WriteFile(filepath.Join(projectDir, "version"), []byte(projectDirVersion), 0o666)
	if versionErr != nil {
		if rmErr := os.RemoveAll(projectDir); rmErr != nil {
			err = multierror.Append(err, errors.Wrap(rmErr, "RemoveAll"))
		}
		err = multierror.Append(err, errors.Wrap(searchErr, "WriteFile (version)"))
	}

	return err
}

// The version stored in e.g. ~/.doctree/index/<project>/version - indicating the version of the
// project directory. If we need to change search indexing, add support for more languages, etc.
// this file is how we'd determine which directories need to be re-indexed / removed.
//
// An incrementing integer. No relation to other version numbers.
const projectDirVersion = "1"

// The version stored in e.g. ~/.doctree/version - indicating the version of the overall data
// directory. If we need to change the directory structure in some way, change the autoindex file
// format, etc. this is what we'd use to determine when to do that.
//
// An incrementing integer. No relation to other version numbers.
const dataDirVersion = "1"

func ensureDataDir(dataDir string) error {
	versionFile := filepath.Join(dataDir, "version")
	_, err := os.Stat(versionFile)
	if os.IsNotExist(err) {
		// Create the directory if needed.
		if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
			return errors.Wrap(err, "MkdirAll")
		}

		// Write the version info.
		return os.WriteFile(versionFile, []byte(dataDirVersion), 0o666)
	}
	if err != nil {
		return err
	}
	return nil
}

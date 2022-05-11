package indexer

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sourcegraph/doctree/doctree/schema"
	sinter "github.com/sourcegraph/doctree/libs/sinter/bindings/sinter-go"
	"github.com/spaolacci/murmur3"
)

// IndexForSearch produces search indexes for the given project, writing them to:
//
// index/<project_name>/search-index.sinter
func IndexForSearch(projectName, indexDataDir string, indexes map[string]*schema.Index) error {
	start := time.Now()
	filter, err := sinter.FilterInit(10_000_000)
	if err != nil {
		return errors.Wrap(err, "FilterInit")
	}
	defer filter.Deinit()

	walkPage := func(p schema.Page, keys []string) []string {
		keys = append(keys, strings.Join(p.SearchKey, ""))

		var walkSection func(s schema.Section)
		walkSection = func(s schema.Section) {
			keys = append(keys, strings.Join(s.SearchKey, ""))

			for _, child := range s.Children {
				walkSection(child)
			}
		}
		for _, section := range p.Sections {
			walkSection(section)
		}
		return keys
	}

	totalNumKeys := 0
	totalNumSearchKeys := 0
	insert := func(language, projectName, pagePath string, searchKeys []string) error {
		absoluteKeys := make([]string, 0, len(searchKeys))
		for _, searchKey := range searchKeys {
			absoluteKeys = append(absoluteKeys, absoluteKey(language, projectName, searchKey))
		}

		totalNumSearchKeys += len(searchKeys)
		fuzzyKeys := FuzzyKeys(absoluteKeys)
		totalNumKeys += len(fuzzyKeys)

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(sinterResult{
			Language:    language,
			ProjectName: projectName,
			SearchKeys:  searchKeys,
			Path:        pagePath,
		}); err != nil {
			return errors.Wrap(err, "Encode")
		}

		if err := filter.Insert(&sinter.SliceIterator{Slice: fuzzyKeys}, buf.Bytes()); err != nil {
			return errors.Wrap(err, "Insert")
		}
		return nil
	}

	for language, index := range indexes {
		for _, lib := range index.Libraries {
			for _, page := range lib.Pages {
				if err := insert(language, projectName, page.Path, walkPage(page, nil)); err != nil {
					return err
				}
				for _, subPage := range page.Subpages {
					if err := insert(language, projectName, page.Path, walkPage(subPage, nil)); err != nil {
						return err
					}
				}
			}
		}
	}

	if err := filter.Index(); err != nil {
		return errors.Wrap(err, "Index")
	}

	indexDataDir, err = filepath.Abs(indexDataDir)
	if err != nil {
		return errors.Wrap(err, "Abs")
	}
	outDir := filepath.Join(indexDataDir, encodeProjectName(projectName))
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		return errors.Wrap(err, "MkdirAll")
	}

	if err := filter.WriteFile(filepath.Join(outDir, "search-index.sinter")); err != nil {
		return errors.Wrap(err, "WriteFile")
	}
	// TODO: This should be in cmd/doctree, not here.
	fmt.Printf("search: indexed %v filter keys (%v search keys) in %v\n", totalNumKeys, totalNumSearchKeys, time.Since(start))

	return nil
}

func absoluteKey(language, projectName, searchKey string) string {
	return strings.Join([]string{language, projectName, searchKey}, " /-/ ")
}

func Search(ctx context.Context, indexDataDir, query string) ([]Result, error) {
	dir, err := ioutil.ReadDir(indexDataDir)
	if os.IsNotExist(err) {
		return []Result{}, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "ReadDir")
	}
	var indexes []string
	for _, info := range dir {
		if info.IsDir() {
			indexes = append(indexes, filepath.Join(indexDataDir, info.Name(), "search-index.sinter"))
		}
	}

	// TODO: return stats about search performance, etc.
	// TODO: query limiting support
	// TODO: support filtering to specific project
	const limit = 100
	out := []Result{}
	for _, sinterFile := range indexes {
		sinterFilter, err := sinter.FilterReadFile(sinterFile)
		if err != nil {
			return nil, errors.Wrap(err, "FilterReadFile: "+sinterFile)
		}

		results, err := sinterFilter.QueryLogicalOr([]uint64{hash(query)})
		if err != nil {
			return nil, errors.Wrap(err, "QueryLogicalOr")
		}
		defer results.Deinit()

		out = append(out, decodeResults(results, query, limit-len(out))...)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

type Result struct {
	Language    string `json:"language"`
	ProjectName string `json:"projectName"`
	SearchKey   string `json:"searchKey"`
	Path        string `json:"path"`
}

type sinterResult struct {
	Language    string   `json:"language"`
	ProjectName string   `json:"projectName"`
	SearchKeys  []string `json:"searchKeys"`
	Path        string   `json:"path"`
}

func decodeResults(results sinter.FilterResults, query string, limit int) []Result {
	var out []Result
decoding:
	for i := 0; i < results.Len(); i++ {
		var result sinterResult
		err := gob.NewDecoder(bytes.NewReader(results.Index(i))).Decode(&result)
		if err != nil {
			panic("illegal sinter result value: " + err.Error())
		}

		for _, searchKey := range result.SearchKeys {
			if match(query, absoluteKey(result.Language, result.ProjectName, searchKey)) {
				out = append(out, Result{
					Language:    result.Language,
					ProjectName: result.ProjectName,
					SearchKey:   searchKey,
					Path:        result.Path,
				})
				if len(out) >= limit {
					break decoding
				}
			}
		}
	}
	return out
}

func match(query, key string) bool {
	for _, part := range append([]string{key}, strings.Split(key, " / ")...) {
		if strings.HasPrefix(part, query) {
			return true
		}
		if strings.HasSuffix(part, query) {
			return true
		}
		lowerQuery := strings.ToLower(query)
		lowerPart := strings.ToLower(part)
		if strings.HasPrefix(lowerPart, lowerQuery) {
			return true
		}
		if strings.HasSuffix(lowerPart, lowerQuery) {
			return true
		}
	}
	return false
}

// TODO: should not be exported, and should take into account language preferences (important
// punctuation list that is language-specific)
//
// TODO: "http.Ge" doesn't match right now, while "http.Get" does. Why?
func FuzzyKeys(keys []string) []uint64 {
	var fuzzyKeys []uint64
	for _, whole := range keys {
		for _, part := range append([]string{whole}, strings.Split(whole, " / ")...) {
			runes := []rune(part)
			fuzzyKeys = append(fuzzyKeys, prefixKeys(runes)...)
			fuzzyKeys = append(fuzzyKeys, suffixKeys(runes)...)
			lowerRunes := []rune(strings.ToLower(part))
			fuzzyKeys = append(fuzzyKeys, prefixKeys(lowerRunes)...)
			fuzzyKeys = append(fuzzyKeys, suffixKeys(lowerRunes)...)
		}
	}
	return fuzzyKeys
}

func prefixKeys(runes []rune) []uint64 {
	var keys []uint64
	var prefix []rune
	for _, r := range runes {
		prefix = append(prefix, r)
		keys = append(keys, hash(string(prefix)))
	}
	return keys
}

func suffixKeys(runes []rune) []uint64 {
	var keys []uint64
	var suffix []rune
	for i := len(runes) - 1; i >= 0; i-- {
		suffix = append([]rune{runes[i]}, suffix...)
		keys = append(keys, hash(string(suffix)))
	}
	return keys
}

func hash(s string) uint64 {
	return murmur3.Sum64([]byte(s))
}

package indexer

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agnivade/levenshtein"
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

	walkPage := func(p schema.Page, keys [][]string, ids []string) ([][]string, []string) {
		keys = append(keys, p.SearchKey)
		ids = append(ids, "")

		var walkSection func(s schema.Section)
		walkSection = func(s schema.Section) {
			keys = append(keys, s.SearchKey)
			ids = append(ids, s.ID)

			for _, child := range s.Children {
				walkSection(child)
			}
		}
		for _, section := range p.Sections {
			walkSection(section)
		}
		return keys, ids
	}

	totalNumKeys := 0
	totalNumSearchKeys := 0
	insert := func(language, projectName, pagePath string, searchKeys [][]string, ids []string) error {
		absoluteKeys := make([][]string, 0, len(searchKeys))
		for _, searchKey := range searchKeys {
			absoluteKeys = append(absoluteKeys, append([]string{language, projectName}, searchKey...))
		}
		if len(absoluteKeys) != len(ids) {
			panic("invariant: len(absoluteKeys) != len(ids)")
		}

		totalNumSearchKeys += len(searchKeys)
		fuzzyKeys := fuzzyKeys(absoluteKeys)
		totalNumKeys += len(fuzzyKeys)

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(sinterResult{
			Language:    language,
			ProjectName: projectName,
			SearchKeys:  searchKeys,
			IDs:         ids,
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
				searchKeys, ids := walkPage(page, nil, nil)
				if err := insert(language, projectName, page.Path, searchKeys, ids); err != nil {
					return err
				}
				for _, subPage := range page.Subpages {
					searchKeys, ids := walkPage(subPage, nil, nil)
					if err := insert(language, projectName, page.Path, searchKeys, ids); err != nil {
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
	const rankedResultLimit = 10000
	const limit = 100
	out := []Result{}
	for _, sinterFile := range indexes {
		sinterFilter, err := sinter.FilterReadFile(sinterFile)
		if err != nil {
			return nil, errors.Wrap(err, "FilterReadFile: "+sinterFile)
		}

		queryKey := strings.FieldsFunc(query, func(r rune) bool { return r == '.' || r == '/' || r == ' ' })
		queryKeyHashes := []uint64{}
		for _, part := range queryKey {
			queryKeyHashes = append(queryKeyHashes, hash(part))
		}
		if len(queryKeyHashes) == 0 {
			// TODO: make QueryLogicalOr handle empty keys set
			queryKeyHashes = []uint64{hash(query)}
		}

		results, err := sinterFilter.QueryLogicalOr(queryKeyHashes)
		if err != nil {
			return nil, errors.Wrap(err, "QueryLogicalOr")
		}
		defer results.Deinit()

		out = append(out, decodeResults(results, queryKey, rankedResultLimit-len(out))...)
		if len(out) >= rankedResultLimit {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type Result struct {
	Language    string  `json:"language"`
	ProjectName string  `json:"projectName"`
	SearchKey   string  `json:"searchKey"`
	Path        string  `json:"path"`
	ID          string  `json:"id"`
	Score       float64 `json:"score"`
}

type sinterResult struct {
	Language    string     `json:"language"`
	ProjectName string     `json:"projectName"`
	SearchKeys  [][]string `json:"searchKeys"`
	IDs         []string   `json:"ids"`
	Path        string     `json:"path"`
}

func decodeResults(results sinter.FilterResults, queryKey []string, limit int) []Result {
	var out []Result
decoding:
	for i := 0; i < results.Len(); i++ {
		var result sinterResult
		err := gob.NewDecoder(bytes.NewReader(results.Index(i))).Decode(&result)
		if err != nil {
			panic("illegal sinter result value: " + err.Error())
		}

		for index, searchKey := range result.SearchKeys {
			absoluteKey := append([]string{result.Language, result.ProjectName}, searchKey...)
			score := match(queryKey, absoluteKey)
			if score > 0.5 {
				out = append(out, Result{
					Language:    result.Language,
					ProjectName: result.ProjectName,
					SearchKey:   strings.Join(searchKey, ""),
					Path:        result.Path,
					ID:          result.IDs[index],
					Score:       score,
				})
				if len(out) >= limit {
					break decoding
				}
			}
		}
	}
	return out
}

func match(queryKey []string, key []string) float64 {
	matchThreshold := 0.75

	score := 0.0
	lastScore := 0.0
	for _, queryPart := range queryKey {
		queryPart = strings.ToLower(queryPart)
		for i, keyPart := range key {
			keyPart = strings.ToLower(keyPart)
			largest := len(queryPart)
			if len(keyPart) > largest {
				largest = len(keyPart)
			}
			// [1.0, 0.0] where 1.0 is exactly equal
			partScore := 1.0 - (float64(levenshtein.ComputeDistance(queryPart, keyPart)) / float64(largest))

			boost := float64(len(key) - i) // Matches on left side of key get more boost
			if partScore > matchThreshold && lastScore > matchThreshold {
				boost *= 2
			}
			finalPartScore := partScore * boost
			score += finalPartScore
			lastScore = finalPartScore
		}
	}
	return score
}

func fuzzyKeys(keys [][]string) []uint64 {
	var fuzzyKeys []uint64
	for _, wholeKey := range keys {
		for _, part := range wholeKey {
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

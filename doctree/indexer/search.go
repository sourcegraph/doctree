package indexer

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sourcegraph/doctree/doctree/schema"
	sinter "github.com/sourcegraph/doctree/libs/sinter/bindings/sinter-go"
	"github.com/spaolacci/murmur3"
)

// IndexForSearch produces search indexes for the given project.
func IndexForSearch(projectName string, indexes map[string]*schema.Index) error {
	filter, err := sinter.FilterInit(10_000_000)
	if err != nil {
		return errors.Wrap(err, "FilterInit")
	}
	defer filter.Deinit()

	walkPage := func(language string, lib schema.Library, p schema.Page, keys []string) []string {
		var walkSection func(s schema.Section)
		walkSection = func(s schema.Section) {
			key := language + " / " + lib.Name + " / " + p.Title + " / " + s.ShortLabel
			keys = append(keys, key)

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
	insert := func(pagePath string, keys []string) error {
		totalNumSearchKeys += len(keys)
		fuzzyKeys := FuzzyKeys(keys)
		totalNumKeys += len(fuzzyKeys)
		if err := filter.Insert(&sinter.SliceIterator{Slice: fuzzyKeys}, []byte(pagePath+"\n\n"+strings.Join(keys, "\n")+"\n\n")); err != nil {
			return errors.Wrap(err, "Insert")
		}
		return nil
	}

	for language, index := range indexes {
		for _, lib := range index.Libraries {
			for _, page := range lib.Pages {
				if err := insert(page.Path, walkPage(language, lib, page, nil)); err != nil {
					return err
				}
				for _, subPage := range page.Subpages {
					if err := insert(page.Path, walkPage(language, lib, subPage, nil)); err != nil {
						return err
					}
				}
			}
		}
	}

	fmt.Printf("search: indexing %v filter keys (%v search keys)\n", totalNumKeys, totalNumSearchKeys)
	if err := filter.Index(); err != nil {
		return errors.Wrap(err, "Index")
	}

	if err := filter.WriteFile("out.sinter"); err != nil {
		return errors.Wrap(err, "WriteFile")
	}

	return nil
}

func Search(query string) ([]string, error) {
	t := time.Now()
	sinterFilter, err := sinter.FilterReadFile("out.sinter")
	if err != nil {
		return nil, errors.Wrap(err, "FilterReadFile")
	}
	fmt.Println("read file:", time.Since(t))
	t = time.Now()

	// TODO: query limiting
	results, err := sinterFilter.QueryLogicalOr([]uint64{hash(query)})
	if err != nil {
		return nil, errors.Wrap(err, "QueryLogicalOr")
	}
	defer results.Deinit()
	fmt.Println("found", results.Len(), "in", time.Since(t))

	for _, result := range Results(results, query) {
		for _, key := range result.Keys {
			fmt.Println(result.Path, "->", key)
		}
	}
	return nil, nil
}

type Result struct {
	Path string
	Keys []string
}

func Results(results sinter.FilterResults, query string) []Result {
	var out []Result
	for i := 0; i < results.Len(); i++ {
		lines := strings.Split(string(results.Index(i)), "\n")
		path := lines[0]
		keys := lines[2:]
		var outKeys []string
		for _, key := range keys {
			if match(query, key) {
				outKeys = append(outKeys, key)
			}
		}
		out = append(out, Result{Path: path, Keys: outKeys})
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
		if strings.HasPrefix(part, lowerQuery) {
			return true
		}
		if strings.HasSuffix(part, lowerQuery) {
			return true
		}
	}
	return false
}

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

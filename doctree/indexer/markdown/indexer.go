// Package markdown provides a doctree indexer implementation for Markdown.
package markdown

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sourcegraph/doctree/doctree/indexer"
	"github.com/sourcegraph/doctree/doctree/schema"
)

func init() {
	indexer.Register(&markdownIndexer{})
}

// Implements the indexer.Language interface.
type markdownIndexer struct{}

func (i *markdownIndexer) Name() schema.Language { return schema.LanguageMarkdown }

func (i *markdownIndexer) Extensions() []string { return []string{"md"} }

func (i *markdownIndexer) IndexDir(ctx context.Context, dir string) (*schema.Index, error) {
	// Find Go sources
	var sources []string
	if err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // error walking dir
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".md" {
				sources = append(sources, path)
			}
		}
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "WalkDir")
	}

	files := 0
	bytes := 0
	pages := []schema.Page{}
	for _, path := range sources {
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, errors.Wrap(err, "ReadFile")
		}
		files += 1
		bytes += len(content)

		pages = append(pages, schema.Page{
			Path:     path,
			Title:    path,
			Detail:   schema.Markdown(content),
			Sections: []schema.Section{},
		})
	}

	return &schema.Index{
		SchemaVersion: schema.LatestVersion,
		Language:      schema.LanguageMarkdown,
		NumFiles:      files,
		NumBytes:      bytes,
		Libraries: []schema.Library{
			{
				Name:        "TODO",
				Repository:  "TODO",
				ID:          "TODO",
				Version:     "TODO",
				VersionType: "TODO",
				Pages:       pages,
			},
		},
	}, nil
}

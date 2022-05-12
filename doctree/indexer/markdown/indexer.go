// Package markdown provides a doctree indexer implementation for Markdown.
package markdown

import (
	"bytes"
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
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
				sources = append(sources, dir+"/"+path)
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

		pages = append(pages, markdownToPage(content, path))
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

func markdownToPage(content []byte, path string) schema.Page {
	// Strip frontmatter out of Markdown documents for now.
	var matter struct {
		Title string   `yaml:"title"`
		Name  string   `yaml:"name"`
		Tags  []string `yaml:"tags"`
	}
	rest, _ := frontmatter.Parse(bytes.NewReader(content), &matter)

	matterTitle := matter.Name
	if matterTitle == "" {
		matterTitle = matter.Title
	}

	primaryContent, childrenSections, firstHeaderName := markdownToSections(rest, 1, matterTitle)

	pageTitle := matterTitle
	if pageTitle == "" {
		pageTitle = firstHeaderName
	}
	searchKey := headerSearchKey(pageTitle, "")
	if pageTitle == "" {
		pageTitle = path
		// no search key for path, leave as empty list.
	}
	return schema.Page{
		Path:      path,
		Title:     pageTitle,
		Detail:    schema.Markdown(primaryContent),
		SearchKey: searchKey,
		Sections:  childrenSections,
	}
}

func markdownToSections(content []byte, level int, pageTitle string) ([]byte, []schema.Section, string) {
	sectionPrefix := []byte(strings.Repeat("#", level) + " ")

	// Group all of the lines separated by a section prefix (e.g. "# heading 1")
	var sectionContent [][][]byte
	var lines [][]byte
	for _, line := range bytes.Split(content, []byte("\n")) {
		if bytes.HasPrefix(line, sectionPrefix) {
			if len(lines) > 0 {
				sectionContent = append(sectionContent, lines)
			}
			lines = nil
		}
		lines = append(lines, line)
	}
	if len(lines) > 0 {
		sectionContent = append(sectionContent, lines)
	}

	// Emit a section for each set of lines we accumulated.
	var (
		primaryContent  []byte
		sections        = []schema.Section{}
		firstHeaderName string
	)
	for _, lines := range sectionContent {
		var name string
		if bytes.HasPrefix(lines[0], sectionPrefix) {
			name = string(bytes.TrimPrefix(lines[0], sectionPrefix))
		}

		if level == 1 && name == "" {
			// This is the content before any heading in a document.
			subPrimaryContent, subChildrenSections, _ := markdownToSections(
				bytes.Join(lines, []byte("\n")),
				level+1,
				pageTitle,
			)
			primaryContent = subPrimaryContent
			sections = append(sections, subChildrenSections...)
			continue
		} else if name == "" {
			primaryContent = bytes.Join(lines, []byte("\n"))
			continue
		}

		if level == 1 && firstHeaderName == "" {
			// This is the first header in a document. Elevate it out.
			firstHeaderName = name
			if pageTitle == "" {
				pageTitle = firstHeaderName
			}
			subPrimaryContent, subChildrenSections, _ := markdownToSections(
				bytes.Join(lines[1:], []byte("\n")),
				level+1,
				pageTitle,
			)
			primaryContent = subPrimaryContent
			sections = append(sections, subChildrenSections...)
			continue
		}

		subPrimaryContent, subChildrenSections, _ := markdownToSections(
			bytes.Join(lines[1:], []byte("\n")),
			level+1,
			pageTitle,
		)

		sections = append(sections, schema.Section{
			ID:         name,
			ShortLabel: name,
			Label:      schema.Markdown(name),
			Detail:     schema.Markdown(subPrimaryContent),
			SearchKey:  headerSearchKey(pageTitle, name),
			Children:   subChildrenSections,
		})
	}

	if len(sections) == 0 && level < 6 {
		nonlinear := false
		for _, line := range bytes.Split(primaryContent, []byte("\n")) {
			if bytes.HasPrefix(line, []byte("#")) {
				nonlinear = true
				break
			}
		}
		if nonlinear {
			return markdownToSections(content, level+1, pageTitle)
		}
	}
	return primaryContent, sections, firstHeaderName
}

func headerSearchKey(pageTitle, section string) []string {
	name := joinNames(pageTitle, section)
	fields := strings.Fields(name)
	searchKey := make([]string, 0, 2+(len(fields)*2))
	searchKey = append(searchKey, []string{"#", " "}...)
	for i, field := range fields {
		searchKey = append(searchKey, field)
		if i != len(fields)-1 {
			searchKey = append(searchKey, " ")
		}
	}
	return searchKey
}

func joinNames(pageTitle, section string) string {
	limit := 60 - len("# ") - len(" > ")
	if len(pageTitle)+len(section) < limit {
		if section != "" {
			return pageTitle + " > " + section
		}
		return pageTitle
	}
	limit /= 2
	if len(pageTitle) > limit {
		pageTitle = pageTitle[:limit]
	}
	if len(section) > limit {
		section = section[:limit]
	}
	if section != "" {
		return pageTitle + " > " + section
	}
	return pageTitle
}

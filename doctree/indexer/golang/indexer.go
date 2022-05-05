// Package golang provides a doctree indexer implementation for Go.
package golang

import (
	"bytes"
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	doc "github.com/slimsag/godocmd"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/sourcegraph/doctree/doctree/indexer"
	"github.com/sourcegraph/doctree/doctree/schema"
)

func init() {
	indexer.Register(&goIndexer{})
}

// Implements the indexer.Language interface.
type goIndexer struct{}

func (i *goIndexer) Name() schema.Language { return schema.LanguageGo }

func (i *goIndexer) Extensions() []string { return []string{"go"} }

func (i *goIndexer) IndexDir(ctx context.Context, dir string) (*schema.Index, error) {
	// Find Go sources
	var sources []string
	if err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // error walking dir
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".go" {
				sources = append(sources, path)
			}
		}
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "WalkDir")
	}

	files := 0
	bytes := 0
	packages := map[string]packageInfo{}
	functionsByPackage := map[string][]schema.Section{}
	for _, path := range sources {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, errors.Wrap(err, "ReadFile")
		}
		files += 1
		bytes += len(content)

		// Parse the file with tree-sitter.
		parser := sitter.NewParser()
		defer parser.Close()
		parser.SetLanguage(golang.GetLanguage())

		tree, err := parser.ParseCtx(ctx, nil, content)
		if err != nil {
			return nil, errors.Wrap(err, "ParseCtx")
		}
		defer tree.Close()

		// Inspect the root node.
		n := tree.RootNode()

		// Package clauses
		var pkgName string
		{
			query, err := sitter.NewQuery([]byte(`
				(
					(comment)* @package_docs
					.
					(package_clause
						(package_identifier) @package_name
					) @package_clause
					(#set-adjacent! @package_docs @package_name)
				)
			`), golang.GetLanguage())
			if err != nil {
				return nil, errors.Wrap(err, "NewQuery")
			}
			defer query.Close()

			cursor := sitter.NewQueryCursor()
			defer cursor.Close()
			cursor.Exec(query, n)

			for {
				match, ok := cursor.NextMatch()
				if !ok {
					break
				}
				captures := getCaptures(query, match)

				pkgDocs := joinCaptures(content, captures["package_docs"], "\n")
				pkgClause := firstCaptureContentOr(content, captures["package_clause"], "")
				_ = pkgClause // TODO: use me!
				pkgName = firstCaptureContentOr(content, captures["package_name"], "")

				if existing, ok := packages[pkgName]; ok {
					if pkgDocs != "" {
						existing.docs += "\n\n"
						existing.docs += pkgDocs
					}
					packages[pkgName] = existing
				} else {
					packages[pkgName] = packageInfo{path: filepath.Dir(path), docs: pkgDocs}
				}
			}
		}

		// Function definitions
		{
			query, err := sitter.NewQuery([]byte(`
				(
					(comment)* @func_docs
					.
					(function_declaration
						name: (identifier) @func_name
						type_parameters: (type_parameter_list)? @func_type_params
						parameters: (parameter_list)? @func_params
						result: (qualified_type package: (package_identifier) name: (type_identifier))? @func_result
					)
				)
			`), golang.GetLanguage())
			if err != nil {
				return nil, errors.Wrap(err, "NewQuery")
			}
			defer query.Close()

			cursor := sitter.NewQueryCursor()
			defer cursor.Close()
			cursor.Exec(query, n)

			for {
				match, ok := cursor.NextMatch()
				if !ok {
					break
				}
				captures := getCaptures(query, match)

				funcDocs := joinCaptures(content, captures["func_docs"], "\n")
				funcName := firstCaptureContentOr(content, captures["func_name"], "")
				funcTypeParams := firstCaptureContentOr(content, captures["func_type_params"], "")
				funcParams := firstCaptureContentOr(content, captures["func_params"], "")
				funcResult := firstCaptureContentOr(content, captures["func_result"], "")

				firstRune := []rune(funcName)[0]
				if string(firstRune) != strings.ToUpper(string(firstRune)) {
					continue // unexported
				}

				funcLabel := schema.Markdown("func " + funcName + funcTypeParams + funcParams)
				if funcResult != "" {
					funcLabel = funcLabel + schema.Markdown(" "+funcResult)
				}
				funcs := functionsByPackage[pkgName]
				funcs = append(funcs, schema.Section{
					ID:         funcName,
					ShortLabel: funcName,
					Label:      funcLabel,
					Detail:     schema.Markdown(cleanDocs(funcDocs, false)),
				})
				functionsByPackage[pkgName] = funcs
			}
		}
	}

	var pages []schema.Page
	for pkgName, pkgInfo := range packages {
		functionsSection := schema.Section{
			ID:         "func",
			ShortLabel: "func",
			Label:      "Functions",
			Category:   true,
			Children:   functionsByPackage[pkgName],
		}

		pages = append(pages, schema.Page{
			Path:     pkgInfo.path,
			Title:    "Package " + pkgName,
			Detail:   schema.Markdown(cleanDocs(pkgInfo.docs, true)),
			Sections: []schema.Section{functionsSection},
		})
	}

	return &schema.Index{
		SchemaVersion: schema.LatestVersion,
		Language:      schema.LanguageGo,
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

func cleanDocs(s string, pkgDocs bool) string {
	var paragraphs []string
	current := ""
	encounteredCopyright := false
	encounteredPackageFoo := false
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, "//go:") {
			continue
		}
		realLine := strings.TrimSpace(strings.TrimPrefix(l, "//"))

		// HACK: https://sourcegraph.slack.com/archives/C03BPE4EGUF/p1651204988551869
		// Tree-sitter query cannot give us newlines between comments, so copyright section ends up
		// as same paragraph as `Package foo` docs.
		if strings.HasPrefix(realLine, "Copyright") {
			encounteredCopyright = true
		}
		if encounteredCopyright && !encounteredPackageFoo {
			if strings.HasPrefix(realLine, "Package ") {
				encounteredPackageFoo = true
			} else {
				continue
			}
		}

		current = strings.TrimSpace(current + " " + realLine)
		if strings.TrimSpace(realLine) == "" && current != "" {
			paragraphs = append(paragraphs, current)
			current = ""
		}
	}
	if current != "" {
		paragraphs = append(paragraphs, current)
	}

	var desirable []string
	for _, p := range paragraphs {
		if strings.HasPrefix(p, "Copyright") {
			continue
		}
		if strings.Contains(p, "DO NOT EDIT") {
			continue
		}
		desirable = append(desirable, p)
	}
	return godocToMarkdown(strings.Join(desirable, "\n\n\n"))
}

func godocToMarkdown(godoc string) string {
	var buf bytes.Buffer
	doc.ToMarkdown(&buf, godoc, nil)
	return buf.String()
}

type packageInfo struct {
	path string
	docs string
}

func firstCaptureContentOr(content []byte, captures []*sitter.Node, defaultValue string) string {
	if len(captures) > 0 {
		return captures[0].Content(content)
	}
	return defaultValue
}

func joinCaptures(content []byte, captures []*sitter.Node, sep string) string {
	var v []string
	for _, capture := range captures {
		v = append(v, capture.Content(content))
	}
	return strings.Join(v, sep)
}

func getCaptures(q *sitter.Query, m *sitter.QueryMatch) map[string][]*sitter.Node {
	captures := map[string][]*sitter.Node{}
	for _, c := range m.Captures {
		cname := q.CaptureNameForId(c.Index)
		captures[cname] = append(captures[cname], c.Node)
	}
	return captures
}

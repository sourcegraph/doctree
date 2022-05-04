// Package golang provides a doctree indexer implementation for Go.
package python

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/sourcegraph/doctree/doctree/indexer"
	"github.com/sourcegraph/doctree/doctree/schema"
)

func init() {
	indexer.Register(&pythonIndexer{})
}

// Implements the indexer.Language interface.
type pythonIndexer struct{}

func (i *pythonIndexer) Name() schema.Language { return schema.LanguagePython }

func (i *pythonIndexer) Extensions() []string { return []string{"py"} }

func (i *pythonIndexer) IndexDir(ctx context.Context, dir string) (*schema.Index, error) {
	// Find Go sources
	var sources []string
	if err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // error walking dir
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".py" {
				sources = append(sources, dir+"/"+path)
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
		path := filepath.Clean(path)
		if strings.Contains(path, "test_") || strings.Contains(path, "_test") || strings.Contains(path, "tests") {
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
		parser.SetLanguage(python.GetLanguage())

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
				(module . (comment)* . (expression_statement . (string) @package_docs)?)
			)
			`), python.GetLanguage())
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

				// Extract package docs and Strip """ from both sides.
				pkgDocs := joinCaptures(content, captures["package_docs"], "\n")
				pkgDocs = sanitizeDocs(pkgDocs)
				pkgName = strings.ReplaceAll(strings.TrimSuffix(path, ".py"), "/", ".")

				if existing, ok := packages[pkgName]; ok {
					if pkgDocs != "" {
						existing.docs += "\n\n"
						existing.docs += pkgDocs
					}
					packages[pkgName] = existing
				} else {
					packages[pkgName] = packageInfo{path: path, docs: pkgDocs}
				}
			}
		}

		// Function definitions
		{
			query, err := sitter.NewQuery([]byte(`
				(
				function_definition
					name: (identifier) @func_name
					parameters: (parameters) @func_params
					return_type: (type)? @func_result
					body: (block . (expression_statement (string) @func_docs)?)
				)
			`), python.GetLanguage())
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
				funcDocs = sanitizeDocs(funcDocs)
				funcName := firstCaptureContentOr(content, captures["func_name"], "")
				funcParams := firstCaptureContentOr(content, captures["func_params"], "")
				funcResult := firstCaptureContentOr(content, captures["func_result"], "")

				if len(funcName) > 0 && funcName[0] == '_' && funcName[len(funcName)-1] != '_' {
					continue // unexported (private function)
				}

				funcLabel := schema.Markdown("def " + funcName + funcParams)
				if funcResult != "" {
					funcLabel = funcLabel + schema.Markdown(" -> "+funcResult)
				}
				funcs := functionsByPackage[pkgName]
				funcs = append(funcs, schema.Section{
					ID:         funcName,
					ShortLabel: funcName,
					Label:      funcLabel,
					Detail:     schema.Markdown(funcDocs),
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
			Detail:   schema.Markdown(pkgInfo.docs),
			Sections: []schema.Section{functionsSection},
		})
	}

	return &schema.Index{
		SchemaVersion: schema.LatestVersion,
		Language:      schema.LanguagePython,
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

func sanitizeDocs(s string) string {
	// TODO: Better rendering of python doctests?
	return strings.TrimSuffix(strings.TrimPrefix(s, "\"\"\""), "\"\"\"")
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

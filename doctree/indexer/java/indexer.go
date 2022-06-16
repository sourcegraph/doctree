package java

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/sourcegraph/doctree/doctree/indexer"
	"github.com/sourcegraph/doctree/doctree/schema"
)

func init() {
	indexer.Register(&javaIndexer{})
}

// Implements the indexer.Language interface.
type javaIndexer struct{}

func (i *javaIndexer) Name() schema.Language { return schema.LanguageJava }

func (i *javaIndexer) Extensions() []string { return []string{"java"} }

func (i *javaIndexer) IndexDir(ctx context.Context, dir string) (*schema.Index, error) {
	// Find Java sources
	var sources []string
	if err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		log.Println(path)
		if d.IsDir() && d.Name() == "test" {
			return filepath.SkipDir
		}
		if err != nil {
			return err // error walking dir
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".java" {
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
	// classesByPackage := map[string][]schema.Section{}
	for _, path := range sources {
		if strings.HasSuffix(path, "Test.java") {
			continue
		}
		dirFS := os.DirFS(dir)
		content, err := fs.ReadFile(dirFS, path)
		if err != nil {
			return nil, errors.Wrap(err, "ReadFile")
		}
		files += 1
		bytes += len(content)

		// Parse the file with tree-sitter.
		parser := sitter.NewParser()
		defer parser.Close()
		parser.SetLanguage(java.GetLanguage())

		tree, err := parser.ParseCtx(ctx, nil, content)
		if err != nil {
			return nil, errors.Wrap(err, "ParseCtx")
		}
		defer tree.Close()

		// Inspect the root node.
		n := tree.RootNode()

		log.Println(n.FieldNameForChild(0))

		// Package clauses
		var pkgName string
		{
			query, err := sitter.NewQuery([]byte(`
			(
				(package_declaration
					(scoped_identifier) @package_name
				) 
			)
			`), java.GetLanguage())
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

				pkgName = firstCaptureContentOr(content, captures["package_name"], "")

				packages[pkgName] = packageInfo{path: path}
			}
		}

		// Class definitions
		{
			query, err := sitter.NewQuery([]byte(`
				(
					(_)*
					(block_comment)* @class_docs
					.
					(class_declaration
						(modifiers)? @class_modifier
					)
				)
			`), java.GetLanguage())
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

				// classDocs := firstCaptureContentOr(content, captures["class_docs"])
				classModifier := firstCaptureContentOr(content, captures["class_modifier"], "")
				// className := firstCaptureContentOr(content, captures["class_name"], "")
				// classTypeParameter := firstCaptureContentOr(content, captures["class_type_parameter"], "")
				// classInterface := firstCaptureContentOr(content, captures["class_interface"], "")

				log.Println(classModifier)
				// log.Println(className)
				// log.Println(classTypeParameter)
				// log.Println(classInterface)

				if strings.Contains(classModifier, "private") {
					continue // unexported
				}

				// Strip annotations

				// classLabel := schema.Markdown("func " + funcName + funcTypeParams + funcParams)

				// classes := classesByPackage[pkgName]
				// classes = append(funcs, schema.Section{
				// 	ID:         className,
				// 	ShortLabel: className,
				// 	Label:      funcLabel,
				// 	Detail:     schema.Markdown(funcDocs),
				// 	SearchKey:  []string{pkgName, ".", funcName},
				// })
				// classesByPackage[pkgName] = classes
			}
		}
	}

	var pages []schema.Page
	for pkgName, pkgInfo := range packages {
		topLevelSections := []schema.Section{}

		pages = append(pages, schema.Page{
			Path:  pkgInfo.path,
			Title: "Package " + pkgName,
			// Detail:    schema.Markdown(pkgInfo.docs),
			SearchKey: []string{pkgName},
			Sections:  topLevelSections,
		})
	}

	return &schema.Index{
		SchemaVersion: schema.LatestVersion,
		Language:      schema.LanguageJava,
		NumFiles:      files,
		NumBytes:      bytes,
		Libraries: []schema.Library{
			{
				Name:        "TODO",
				ID:          "TODO",
				Version:     "TODO",
				VersionType: "TODO",
				Pages:       pages,
			},
		},
	}, nil
}

type packageInfo struct {
	path string
}

func firstCaptureContentOr(content []byte, captures []*sitter.Node, defaultValue string) string {
	if len(captures) > 0 {
		return captures[0].Content(content)
	}
	return defaultValue
}

// func joinCaptures(content []byte, captures []*sitter.Node, sep string) string {
// 	var v []string
// 	for _, capture := range captures {
// 		v = append(v, capture.Content(content))
// 	}
// 	return strings.Join(v, sep)
// }

func getCaptures(q *sitter.Query, m *sitter.QueryMatch) map[string][]*sitter.Node {
	captures := map[string][]*sitter.Node{}
	for _, c := range m.Captures {
		cname := q.CaptureNameForId(c.Index)
		captures[cname] = append(captures[cname], c.Node)
	}
	return captures
}

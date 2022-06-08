// Package typescript provides a doctree indexer implementation for TypeScript.
package typescript

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	"github.com/sourcegraph/doctree/doctree/indexer"
	"github.com/sourcegraph/doctree/doctree/schema"
)

func init() {
	indexer.Register(&typescriptIndexer{})
}

// Implements the indexer.Language interface.
type typescriptIndexer struct{}

func (i *typescriptIndexer) Name() schema.Language { return schema.LanguageTypeScript }

func (i *typescriptIndexer) Extensions() []string { return []string{"ts"} }

func (i *typescriptIndexer) IndexDir(ctx context.Context, dir string) (*schema.Index, error) {
	// Find TypeScript sources
	var sources []string
	if err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // error walking dir
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".ts" {
				sources = append(sources, path)
			}
		}
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "WalkDir")
	}

	files := 0
	bytes := 0
	mods := map[string]moduleInfo{}
	functionsByMod := map[string][]schema.Section{}
	classesByMod := map[string][]schema.Section{}

	for _, path := range sources {
		if strings.Contains(path, "test_") || strings.Contains(path, "_test") || strings.Contains(path, "tests") || strings.Contains(path, "node_modules") {
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
		parser.SetLanguage(typescript.GetLanguage())

		tree, err := parser.ParseCtx(ctx, nil, content)
		if err != nil {
			return nil, errors.Wrap(err, "ParseCtx")
		}
		defer tree.Close()

		// Inspect the root node.
		n := tree.RootNode()

		// Modules
		var modName string = strings.ReplaceAll(strings.TrimSuffix(path, "."), "/", ".")
		{
			query, err := sitter.NewQuery([]byte(programQuery), typescript.GetLanguage())
			if err != nil {
				return nil, errors.Wrap(err, "NewQuery")
			}
			defer query.Close()

			cursor := sitter.NewQueryCursor()
			defer cursor.Close()
			cursor.Exec(query, n)

			mods[modName] = moduleInfo{path: path, docs: ""}

			for {
				match, ok := cursor.NextMatch()
				if !ok {
					break
				}
				captures := getCaptures(query, match)

				modDocs := joinCaptures(content, captures["module_docs"], "\n")
				modDocs = sanitizeDocs(modDocs)

				// mods[modName] = moduleInfo{path: path, docs: modDocs}
				mods[modName] = moduleInfo{path: path}
			}
		}
	}

	// Functions
	{
		for _, mod := range mods {
			// NEXT: setup go code intel properly, then resume
		}
	}

	// TODO: classes and functions and all that jazz

	var pages []schema.Page
	for modName, moduleInfo := range mods {
		functionsSection := schema.Section{
			ID:         "func",
			ShortLabel: "func",
			Label:      "Functions",
			SearchKey:  []string{},
			Category:   true,
			Children:   functionsByMod[modName],
		}

		classesSection := schema.Section{
			ID:         "class",
			ShortLabel: "class",
			Label:      "Classes",
			SearchKey:  []string{},
			Category:   true,
			Children:   classesByMod[modName],
		}

		pages = append(pages, schema.Page{
			Path:      moduleInfo.path,
			Title:     "Module " + modName,
			Detail:    schema.Markdown(moduleInfo.docs),
			SearchKey: []string{modName},
			Sections:  []schema.Section{functionsSection, classesSection},
		})
	}

	return &schema.Index{
		SchemaVersion: schema.LatestVersion,
		Language:      schema.LanguageTypeScript,
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

type moduleInfo struct {
	path string
	docs string
}

// TODO: remove
func sanitizeDocs(s string) string {
	if strings.HasPrefix(s, "//") {
		return strings.TrimPrefix(s, "//")
	} else if strings.HasPrefix(s, "/*") {
		return strings.TrimSuffix(strings.TrimPrefix(s, "/*"), "*/")
	}
	return s
}

var patternsToElide = []*regexp.Regexp{
	regexp.MustCompile(`^/\*+\s?`),
	regexp.MustCompile(`\s?\*+/$`),
	regexp.MustCompile(`^\*\s?`),
	regexp.MustCompile(`^//\s?`),
}

func cleanTypeScriptComment(raw string) string {
	s := raw
	s = strings.TrimSpace(s)
	lines := strings.Split(raw, "\n")
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		for _, p := range patternsToElide {
			line = p.ReplaceAllString(line, "")
		}
		lines[i] = line
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
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

var (
	programQuery = `
(program
	.
	(comment)* @comments
) @program`
	functionQuery = `
(program
	(_)?
	(comment)? @doc
	.
	[
		(lexical_declaration
			(variable_declarator
				(identifier) @name
				[
					(arrow_function)
					(function)
				]
			)
		)
		(function_declaration
			(identifier) @name
		)
	] @func
)`
	exportedVarQuery = `
(program
	(_)?
	(comment)? @doc
	.
	(export_statement
		[
			declaration: (lexical_declaration
				(variable_declarator
					name: (identifier) @name
					[
						(arrow_function) @func_def
						(function) @func_def
						(_) @other
					]
				)
			)
			(function_declaration
				(identifier) @name
			) @func_def
			(class_declaration
				name: (type_identifier) @name
			) @class_def
		] @decl
	) @export
)`
	classQuery = `
(program
	(_)?
	(comment)? @doc
	.
	(class_declaration
		name: (type_identifier) @name
	) @class
)`
)

var tsQueryTree = &SymbolQuery{
	Query: programQuery,
	NewSymbol: func(match *sitter.QueryMatch, c map[string][]*sitter.Node, namespace string, code []byte) *Symbol {
		doc := strings.Join(getContents(c["comments"], code, cleanTypeScriptComment), "\n")
		return NewSymbol(namespace, namespace, "module", doc, c["program"][0])
	},
	Children: []*SymbolQuery{
		{
			Query: functionQuery,
			NewSymbol: func(match *sitter.QueryMatch, c map[string][]*sitter.Node, namespace string, code []byte) *Symbol {
				doc := cleanTypeScriptComment(getContent(c["doc"], code))
				name := getContent(c["name"], code)
				return NewSymbol(path.Join(namespace, name), name, "func", doc, c["func"][0])
			},
		},
		{
			Query: exportedVarQuery,
			NewSymbol: func(match *sitter.QueryMatch, c map[string][]*sitter.Node, namespace string, code []byte) *Symbol {
				if _, isFunc := c["func_def"]; isFunc {
					doc := cleanTypeScriptComment(getContent(c["doc"], code))
					name := getContent(c["name"], code)
					_, exported := c["export"]
					return &Symbol{
						URI:      path.Join(namespace, name),
						Name:     name,
						Type:     "func",
						Doc:      doc,
						Exported: exported,
						Node:     c["decl"][0],
					}
				} else if _, isClass := c["class_def"]; isClass {
					doc := cleanTypeScriptComment(getContent(c["doc"], code))
					name := getContent(c["name"], code)
					_, exported := c["export"]
					return &Symbol{
						URI:      path.Join(namespace, name),
						Name:     name,
						Type:     "class",
						Doc:      doc,
						Exported: exported,
						Node:     c["class_def"][0],
					}
				} else {
					doc := cleanTypeScriptComment(getContent(c["doc"], code))
					name := getContent(c["name"], code)
					_, exported := c["export"]
					return &Symbol{
						URI:      path.Join(namespace, name),
						Name:     name,
						Type:     "var",
						Doc:      doc,
						Exported: exported,
						Node:     c["decl"][0],
					}
				}
			},
		},
		{
			Query: classQuery,
			NewSymbol: func(match *sitter.QueryMatch, c map[string][]*sitter.Node, namespace string, code []byte) *Symbol {
				doc := cleanTypeScriptComment(getContent(c["doc"], code))
				name := getContent(c["name"], code)
				_, exported := c["export"]
				return &Symbol{
					URI:      path.Join(namespace, name),
					Name:     name,
					Type:     "class",
					Doc:      doc,
					Exported: exported,
					Node:     c["class"][0],
				}
			},
		},
	},
}

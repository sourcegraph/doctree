// Package zig provides a doctree indexer implementation for Zig.
package zig

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	zig "github.com/slimsag/tree-sitter-zig/bindings/go"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/sourcegraph/doctree/doctree/indexer"
	"github.com/sourcegraph/doctree/doctree/schema"
)

func init() {
	indexer.Register(&zigIndexer{})
}

// Implements the indexer.Language interface.
type zigIndexer struct{}

func (i *zigIndexer) Name() schema.Language { return schema.LanguageZig }

func (i *zigIndexer) Extensions() []string { return []string{"zig"} }

func (i *zigIndexer) IndexDir(ctx context.Context, dir string) (*schema.Index, error) {
	// Find Zig sources
	var sources []string
	dirFS := os.DirFS(dir)
	if err := fs.WalkDir(dirFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // error walking dir
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".zig" {
				sources = append(sources, path)
			}
		}
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "WalkDir")
	}

	deps := depGraph{}
	for _, path := range sources {
		content, err := fs.ReadFile(dirFS, path)
		if err != nil {
			return nil, errors.Wrap(err, "ReadFile")
		}

		// Parse the file with tree-sitter.
		parser := sitter.NewParser()
		defer parser.Close()
		parser.SetLanguage(zig.GetLanguage())

		tree, err := parser.ParseCtx(ctx, nil, content)
		if err != nil {
			return nil, errors.Wrap(err, "ParseCtx")
		}
		defer tree.Close()

		// Inspect the root node.
		n := tree.RootNode()

		// Variable declarations
		{
			query, err := sitter.NewQuery([]byte(`
				(
					"pub"? @pub
					.
					(TopLevelDecl
						(VarDecl
							variable_type_function:
							(IDENTIFIER) @var_name
							(ErrorUnionExpr
								(SuffixExpr
									(BUILTINIDENTIFIER)
									(FnCallArguments
										(ErrorUnionExpr
											(SuffixExpr
												(STRINGLITERALSINGLE)
											)
										)
									)
								)
							) @var_expr
						)
					)
				)
			`), zig.GetLanguage())
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

				pub := firstCaptureContentOr(content, captures["pub"], "") == "pub"
				varName := firstCaptureContentOr(content, captures["var_name"], "")
				varExpr := firstCaptureContentOr(content, captures["var_expr"], "")

				if strings.HasPrefix(varExpr, "@import(") {
					importPath := strings.TrimSuffix(strings.TrimPrefix(varExpr, `@import("`), `")`)
					deps.insert(path, pub, varName, importPath)
				}
			}
		}
	}
	deps.build()

	files := 0
	bytes := 0
	functionsByFile := map[string][]schema.Section{}
	for _, path := range sources {
		content, err := fs.ReadFile(dirFS, path)
		if err != nil {
			return nil, errors.Wrap(err, "ReadFile")
		}
		files += 1
		bytes += len(content)

		// Parse the file with tree-sitter.
		parser := sitter.NewParser()
		defer parser.Close()
		parser.SetLanguage(zig.GetLanguage())

		tree, err := parser.ParseCtx(ctx, nil, content)
		if err != nil {
			return nil, errors.Wrap(err, "ParseCtx")
		}
		defer tree.Close()

		// Inspect the root node.
		n := tree.RootNode()

		// Variable declarations
		{
			query, err := sitter.NewQuery([]byte(`
				(
					(container_doc_comment)* @container_docs
					.
					"pub"? @pub
					.
					(TopLevelDecl
						(VarDecl
							variable_type_function:
							(IDENTIFIER) @var_name
							(ErrorUnionExpr
								(SuffixExpr
									(BUILTINIDENTIFIER)
									(FnCallArguments
										(ErrorUnionExpr
											(SuffixExpr
												(STRINGLITERALSINGLE)
											)
										)
									)
								)
							) @var_expr
						)
					)
				)
			`), zig.GetLanguage())
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

				containerDocs := firstCaptureContentOr(content, captures["container_docs"], "")
				pub := firstCaptureContentOr(content, captures["pub"], "") == "pub"
				varName := firstCaptureContentOr(content, captures["var_name"], "")
				varExpr := firstCaptureContentOr(content, captures["var_expr"], "")

				_ = containerDocs
				_ = pub
				_ = varName
				_ = varExpr
				// TODO: emit variables/constants section
			}
		}

		// Function definitions
		{
			// TODO: This query is incorrectly pulling out methods from nested struct definitions.
			// So we end up with a flat hierarchy of types - that's very bad. It also means we don't
			// accurately pick up when a method is part of a parent type.
			query, err := sitter.NewQuery([]byte(`
				(
					(doc_comment)* @func_docs
					.
					"pub"? @pub
					.
					(TopLevelDecl
						(FnProto
							function:
							(IDENTIFIER) @func_name
							(ParamDeclList) @func_params
							(ErrorUnionExpr
								(SuffixExpr
									(BuildinTypeExpr)
								)
							) @func_result
						)
					)
				)
			`), zig.GetLanguage())
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

				pub := firstCaptureContentOr(content, captures["pub"], "")
				if pub != "pub" {
					continue
				}

				funcDocs := firstCaptureContentOr(content, captures["func_docs"], "")
				funcName := firstCaptureContentOr(content, captures["func_name"], "")
				funcParams := firstCaptureContentOr(content, captures["func_params"], "")
				funcResult := firstCaptureContentOr(content, captures["func_result"], "")

				accessiblePath := deps.fileToAccessiblePath[path]
				var searchKey []string
				if accessiblePath == "" {
					searchKey = []string{funcName}
				} else {
					searchKey = []string{accessiblePath, ".", funcName}
				}
				functionsByFile[path] = append(functionsByFile[path], schema.Section{
					ID:         funcName,
					ShortLabel: funcName,
					Label:      schema.Markdown(funcName + funcParams + " " + funcResult),
					Detail:     schema.Markdown(docsToMarkdown(funcDocs)),
					SearchKey:  searchKey,
				})
			}
		}
	}

	var pages []schema.Page
	for path, functions := range functionsByFile {
		functionsSection := schema.Section{
			ID:         "fn",
			ShortLabel: "fn",
			Label:      "Functions",
			Category:   true,
			SearchKey:  []string{},
			Children:   functions,
		}

		pages = append(pages, schema.Page{
			Path:      path,
			Title:     path,
			Detail:    schema.Markdown("TODO"),
			SearchKey: []string{path},
			Sections:  []schema.Section{functionsSection},
		})
	}

	return &schema.Index{
		SchemaVersion: schema.LatestVersion,
		Language:      schema.LanguageZig,
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

type importRecord struct {
	path       string
	pub        bool
	name       string
	importPath string
}

type depGraph struct {
	records              []importRecord
	fileToAccessiblePath map[string]string
}

func (d *depGraph) insert(path string, pub bool, name, importPath string) {
	d.records = append(d.records, importRecord{path, pub, name, importPath})
	if d.fileToAccessiblePath == nil {
		d.fileToAccessiblePath = map[string]string{}
	}
	d.fileToAccessiblePath[path] = ""
}

func (d *depGraph) build() {
	for filePath := range d.fileToAccessiblePath {
		path := d.collect(filePath, nil, map[string]struct{}{})
		d.fileToAccessiblePath[filePath] = strings.Join(path, ".")
		// fmt.Println(filePath, strings.Join(path, "."))
	}
}

func (d *depGraph) collect(targetPath string, result []string, cyclic map[string]struct{}) []string {
	for _, record := range d.records {
		if !record.pub {
			continue
		}
		if strings.HasSuffix(record.importPath, ".zig") {
			record.importPath = path.Join(path.Dir(record.path), record.importPath)
		}
		if record.importPath == targetPath {
			if _, ok := cyclic[record.path]; ok {
				return result
			}
			cyclic[record.path] = struct{}{}
			return d.collect(record.path, append([]string{record.name}, result...), cyclic)
		}
	}
	return result
}

func docsToMarkdown(docs string) string {
	var out []string
	for _, s := range strings.Split(docs, "\n") {
		if strings.HasPrefix(s, "/// ") {
			out = append(out, strings.TrimPrefix(s, "/// "))
			continue
		} else if strings.HasPrefix(s, "//! ") {
			out = append(out, strings.TrimPrefix(s, "//! "))
			continue
		}
		out = append(out, strings.TrimPrefix(s, "// "))
	}
	return strings.Join(out, "\n")
}

func firstCaptureContentOr(content []byte, captures []*sitter.Node, defaultValue string) string {
	if len(captures) > 0 {
		return captures[0].Content(content)
	}
	return defaultValue
}

func getCaptures(q *sitter.Query, m *sitter.QueryMatch) map[string][]*sitter.Node {
	captures := map[string][]*sitter.Node{}
	for _, c := range m.Captures {
		cname := q.CaptureNameForId(c.Index)
		captures[cname] = append(captures[cname], c.Node)
	}
	return captures
}

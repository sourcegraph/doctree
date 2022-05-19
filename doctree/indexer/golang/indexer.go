// Package golang provides a doctree indexer implementation for Go.
package golang

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
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
	typesByPackage := map[string][]schema.Section{}
	functionsByPackage := map[string][]schema.Section{}
	for _, path := range sources {
		if strings.HasSuffix(path, "_test.go") {
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

				pkgClause := firstCaptureContentOr(content, captures["package_clause"], "")
				pkgDocs := commentsToMarkdown(content, extractPackageDocs(captures["package_docs"], captures["package_clause"]))
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

				funcDocs := commentsToMarkdown(content, captures["func_docs"])
				funcName := firstCaptureContentOr(content, captures["func_name"], "")
				funcTypeParams := firstCaptureContentOr(content, captures["func_type_params"], "")
				funcParams := firstCaptureContentOr(content, captures["func_params"], "")
				funcResult := firstCaptureContentOr(content, captures["func_result"], "")

				firstRune := []rune(funcName)[0]
				if string(firstRune) != strings.ToUpper(string(firstRune)) || string(firstRune) == "_" {
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
					Detail:     schema.Markdown(funcDocs),
					SearchKey:  []string{pkgName, ".", funcName},
				})
				functionsByPackage[pkgName] = funcs
			}
		}

		// Type declarations
		{
			query, err := sitter.NewQuery([]byte(`
				(source_file
					(_)?
					(comment)* @type_docs
					.
					(type_declaration
						(type_spec
							name: (type_identifier) @type_name
							type: [
								(struct_type) @type_struct
								(interface_type) @type_interface
								(function_type) @type_func

								(generic_type) @type_other
								(qualified_type) @type_other
								(pointer_type) @type_other
								(array_type) @type_other
								(slice_type) @type_other
								(map_type) @type_other
								(channel_type) @type_other
							]
						)
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

				typeDocs := commentsToMarkdown(content, captures["type_docs"])
				typeName := firstCaptureContentOr(content, captures["type_name"], "")

				typeStruct := firstCaptureContentOr(content, captures["type_struct"], "")
				typeInterface := firstCaptureContentOr(content, captures["type_interface"], "")
				typeFunc := firstCaptureContentOr(content, captures["type_func"], "")
				typeOther := firstCaptureContentOr(content, captures["type_other"], "")

				firstRune := []rune(typeName)[0]
				if string(firstRune) != strings.ToUpper(string(firstRune)) || string(firstRune) == "_" {
					continue // unexported
				}

				var typeLabel schema.Markdown
				var typeDefinition string
				if typeStruct != "" {
					typeLabel = schema.Markdown(fmt.Sprintf("type %s struct", typeName))
					typeDefinition = fmt.Sprintf("type %s %s", typeName, typeStruct)
				} else if typeInterface != "" {
					typeLabel = schema.Markdown(fmt.Sprintf("type %s interface", typeName))
					typeDefinition = fmt.Sprintf("type %s %s", typeName, typeInterface)
				} else if typeFunc != "" {
					typeLabel = schema.Markdown(fmt.Sprintf("type %s func", typeName))
					typeDefinition = fmt.Sprintf("type %s %s", typeName, typeFunc)
				} else {
					firstLine := strings.Split(typeOther, "\n")[0]
					typeLabel = schema.Markdown(fmt.Sprintf("type %s %s", typeName, firstLine))
					typeDefinition = fmt.Sprintf("type %s %s", typeName, typeOther)
				}

				types := typesByPackage[pkgName]
				types = append(types, schema.Section{
					ID:         typeName,
					ShortLabel: typeName,
					Label:      typeLabel,
					Detail:     schema.Markdown(fmt.Sprintf("```go\n%s\n```\n\n%s", typeDefinition, typeDocs)),
					SearchKey:  []string{pkgName, ".", typeName},
				})
				typesByPackage[pkgName] = types
			}
		}
	}

	var pages []schema.Page
	for pkgName, pkgInfo := range packages {
		typesSection := schema.Section{
			ID:         "type",
			ShortLabel: "type",
			Label:      "Types",
			Category:   true,
			SearchKey:  []string{},
			Children:   typesByPackage[pkgName],
		}

		functionsSection := schema.Section{
			ID:         "func",
			ShortLabel: "func",
			Label:      "Functions",
			Category:   true,
			SearchKey:  []string{},
			Children:   functionsByPackage[pkgName],
		}

		pages = append(pages, schema.Page{
			Path:      pkgInfo.path,
			Title:     "Package " + pkgName,
			Detail:    schema.Markdown(pkgInfo.docs),
			SearchKey: []string{pkgName},
			Sections: []schema.Section{
				typesSection,
				functionsSection,
			},
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

func commentsToMarkdown(content []byte, captures []*sitter.Node) string {
	// Turn /* multiline */ and // single line comments into plain text.
	var joined []string
	for _, capture := range captures {
		text := capture.Content(content)
		if strings.HasPrefix(text, "/*") {
			joined = append(joined, strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/"))
		} else {
			var lines []string
			for _, l := range strings.Split(text, "\n") {
				realLine := strings.TrimPrefix(strings.TrimPrefix(l, "//"), " ")
				lines = append(lines, realLine)
			}
			joined = append(joined, strings.Join(lines, "\n"))
		}
	}
	s := strings.Join(joined, "\n")

	// Convert godoc strings into Markdown.
	markdown := godocToMarkdown(s)

	// HACK: godocToMarkdown emits an extra blank line before closing code blocks, remove it.
	markdown = strings.Replace(markdown, "\n```\n", "```\n", -1)
	return strings.TrimSpace(markdown)
}

// The comments preceding a package clause are not always considered package docs. Only those
// immediately preceding the package clause are, there may be a blank line separating a copyright
// header in the file from the clause in which case they are not considered package docs. This
// function picks out the last consecutive set of captured comments, and then returns that set only
// if it directly precedes the clause.
func extractPackageDocs(captures, pkgClauseCaptures []*sitter.Node) []*sitter.Node {
	if len(pkgClauseCaptures) == 0 {
		return nil
	}
	clause := pkgClauseCaptures[0].StartPoint()
	var (
		pkgDocs []*sitter.Node
		lastRow uint32
	)
	for _, capture := range captures {
		point := capture.EndPoint()
		if point.Row != lastRow+1 {
			pkgDocs = pkgDocs[:0]
		}
		pkgDocs = append(pkgDocs, capture)
	}
	if len(pkgDocs) == 0 || pkgDocs[len(pkgDocs)-1].EndPoint().Row != clause.Row-1 {
		return nil
	}
	return pkgDocs
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

func getCaptures(q *sitter.Query, m *sitter.QueryMatch) map[string][]*sitter.Node {
	captures := map[string][]*sitter.Node{}
	for _, c := range m.Captures {
		cname := q.CaptureNameForId(c.Index)
		captures[cname] = append(captures[cname], c.Node)
	}
	return captures
}

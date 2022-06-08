// Package typescript provides a doctree indexer implementation for TypeScript.
package typescript

import (
	"context"
	"io/fs"
	"log"
	"os"
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
		// DEBUG
		if !strings.Contains(path, "renderer.ts") {
			continue
		}

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
		rootNode := tree.RootNode()

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
			cursor.Exec(query, rootNode)

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

		log.Printf(treeToString(content, rootNode))

		// Functions
		{
			query, err := sitter.NewQuery([]byte(functionQuery), typescript.GetLanguage())
			if err != nil {
				return nil, errors.Wrap(err, "NewQuery(funcQuery)")
			}
			defer query.Close()

			cursor := sitter.NewQueryCursor()
			defer cursor.Close()
			cursor.Exec(query, rootNode)

			for {
				match, ok := cursor.NextMatch()
				if !ok {
					break
				}
				captures := getCaptures(query, match)
				funcDocs := firstCaptureContentOr(content, captures["func_docs"], "") // TODO
				funcDocs = sanitizeDocs(funcDocs)
				funcName := firstCaptureContentOr(content, captures["func_name"], "")
				funcParams := firstCaptureContentOr(content, captures["func_params"], "")

				funcLabel := schema.Markdown("function " + funcName + funcParams)

				log.Printf("# funcName: %s", funcName)

				functionsByMod[modName] = append(functionsByMod[modName], schema.Section{
					ID:         funcName,
					ShortLabel: funcName,
					Label:      funcLabel,
					Detail:     schema.Markdown(funcDocs),
					SearchKey:  append([]string{modName}, ".", funcName),
				})
			}
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
	node *sitter.Node
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
	(comment)* @module_docs
) @program`

	// TODO: func_params for arrow function syntax
	// TODO: revisit lexical_declaration syntax; maybe remove exported funcs
	functionQuery = `
(program
	(_)?
	(comment)? @func_docs
	.
	[
		(lexical_declaration
			(variable_declarator
				(identifier) @func_name
				[
					(arrow_function)
					(function)
				]
			)
		)
		(function_declaration
			name: (identifier) @func_name
			parameters: (formal_parameters) @func_params
		)
		(export_statement
			(function_declaration
				name: (identifier) @func_name
				parameters: (formal_parameters) @func_params
			)
		)
	] @func
)`
	exportedVarQuery = `
(program
	(_)?
	(comment)? @var_docs
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

func firstCaptureContentOr(content []byte, captures []*sitter.Node, defaultValue string) string {
	if len(captures) > 0 {
		return captures[0].Content(content)
	}
	return defaultValue
}

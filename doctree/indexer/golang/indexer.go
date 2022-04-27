package golang

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
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
		ext := filepath.Ext(path)
		if ext == ".go" {
			sources = append(sources, path)
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
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
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
		query, err := sitter.NewQuery([]byte(`
			(source_file
				(package_clause
					(package_identifier) @package_name
				) @package_clause
				(function_declaration
					name: (identifier) @func_name
					type_parameters: (type_parameter_list)? @func_type_params
					parameters: (parameter_list)? @func_params
					result: (qualified_type package: (package_identifier) name: (type_identifier))? @func_result
				)
			) @file
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
			_ = pkgClause // TODO: use me!
			pkgName := firstCaptureContentOr(content, captures["package_name"], "")
			funcName := firstCaptureContentOr(content, captures["func_name"], "")
			funcTypeParams := firstCaptureContentOr(content, captures["func_type_params"], "")
			funcParams := firstCaptureContentOr(content, captures["func_params"], "")
			funcResult := firstCaptureContentOr(content, captures["func_result"], "")

			packages[pkgName] = packageInfo{path: filepath.Dir(path)}

			funcLabel := schema.Markdown("func " + funcName + funcTypeParams + funcParams)
			if funcResult != "" {
				funcLabel = funcLabel + schema.Markdown(" "+funcResult)
			}
			funcs := functionsByPackage[pkgName]
			funcs = append(funcs, schema.Section{
				ID:         funcName,
				ShortLabel: funcName,
				Label:      funcLabel,
				Detail:     "TODO",
			})
			functionsByPackage[pkgName] = funcs
		}
	}

	var pages []schema.Page
	for pkgName, pkgInfo := range packages {
		functionsSection := schema.Section{
			ID:         "func",
			ShortLabel: "func",
			Label:      "Functions",
			Detail:     schema.Markdown("Package " + pkgName + " provides ...TODO..."),
			Children:   functionsByPackage[pkgName],
		}

		pages = append(pages, schema.Page{
			Path:     pkgInfo.path,
			Title:    "Package " + pkgName,
			Sections: []schema.Section{functionsSection},
		})
	}

	return &schema.Index{
		SchemaVersion: schema.LatestVersion,
		Language:      schema.LanguageGo,
		NumFiles:      files,
		NumBytes:      bytes,
		Library: schema.Library{
			Name:        "TODO",
			Repository:  "TODO",
			ID:          "TODO",
			Version:     "TODO",
			VersionType: "TODO",
			Pages:       pages,
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

func getCaptures(q *sitter.Query, m *sitter.QueryMatch) map[string][]*sitter.Node {
	captures := map[string][]*sitter.Node{}
	for _, c := range m.Captures {
		cname := q.CaptureNameForId(c.Index)
		captures[cname] = append(captures[cname], c.Node)
	}
	return captures
}

// Package javascript provides a doctree indexer implementation for Javascript.
package javascript

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	jsdoc "github.com/DaivikDave/tree-sitter-jsdoc/bindings/go"
	"github.com/pkg/errors"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/sourcegraph/doctree/doctree/indexer"
	"github.com/sourcegraph/doctree/doctree/schema"
)

func init() {
	indexer.Register(&javascriptIndexer{})
}

// Implements the indexer.Language interface.
type javascriptIndexer struct{}

func (i *javascriptIndexer) Name() schema.Language { return schema.LanguageJavaScript }

func (i *javascriptIndexer) Extensions() []string { return []string{"js"} }

func (i *javascriptIndexer) IndexDir(ctx context.Context, dir string) (*schema.Index, error) {
	// Find JavaScript sources
	var sources []string
	if err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // error walking dir
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".js" {
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
		parser.SetLanguage(javascript.GetLanguage())

		tree, err := parser.ParseCtx(ctx, nil, content)
		if err != nil {
			return nil, errors.Wrap(err, "ParseCtx")
		}
		defer tree.Close()

		// Inspect the root node.
		n := tree.RootNode()

		// Module clauses
		var modName string = strings.ReplaceAll(strings.TrimSuffix(path, "."), "/", ".")
		{
			query, err := sitter.NewQuery([]byte(`
			(
				(comment)* @module_docs
				.
				(import_statement)*
				)
			`), javascript.GetLanguage())
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

				mods[modName] = moduleInfo{path: path, docs: modDocs}
			}
		}

		funcDefQuery := functionDefinitionQuery()

		// Function definitions
		{
			modFunctions, err := getFunctions(n, content, funcDefQuery, []string{modName})
			if err != nil {
				return nil, err
			}

			functionsByMod[modName] = modFunctions

		}

		// Classes and their methods
		{
			// Find out all the classes
			query, err := sitter.NewQuery([]byte(`
			(
				[
					(
						(comment)* @class_docs
						.
						(class_declaration
							name: (identifier) @class_name
							(class_heritage (identifier) @superclasses)? 
						 	body: (class_body) 
						) @class_declaration
					)
					(
						(comment)* @class_docs
						.
						(export_statement
							value: (class
								name: (identifier) @class_name
								(class_heritage (identifier) @superclasses)? 
									body: (class_body) 
							) @class_declaration
						)	    
					)
				]
			)
			`), javascript.GetLanguage())
			if err != nil {
				return nil, errors.Wrap(err, "NewQuery")
			}
			defer query.Close()

			cursor := sitter.NewQueryCursor()
			defer cursor.Close()
			cursor.Exec(query, n)

			// Iterate over the classes
			for {
				match, ok := cursor.NextMatch()
				if !ok {
					break
				}
				captures := getCaptures(query, match)

				className := firstCaptureContentOr(content, captures["class_name"], "")
				superClasses := firstCaptureContentOr(content, captures["superclasses"], "")
				classDocs := firstCaptureContentOr(content, captures["class_docs"], "\n")
				classDocs = sanitizeDocs(classDocs)

				classLabel := schema.Markdown("class " + className + superClasses)
				classes := classesByMod[modName]

				// Extract class methods:
				classFuncQuery := `
							(_
								(comment)* @func_docs
								.
								member: (method_definition
									name: (property_identifier) @func_name
									parameters: (formal_parameters) @func_params
								)    
							)
				`

				var classMethods []schema.Section
				classBodyNodes := captures["class_declaration"]
				if len(classBodyNodes) > 0 {
					classMethods, err = getFunctions(
						classBodyNodes[0], content, classFuncQuery,
						[]string{modName, ".", className},
					)
					if err != nil {
						return nil, err
					}
				}
				classes = append(classes, schema.Section{
					ID:         className,
					ShortLabel: className,
					Label:      classLabel,
					Detail:     schema.Markdown(classDocs),
					SearchKey:  []string{modName, ".", className},
					Children:   classMethods,
				})
				classesByMod[modName] = classes
			}
		}
	}

	var pages []schema.Page
	for modName, moduleInfo := range mods {
		sections := []schema.Section{}

		if funcSections, ok := functionsByMod[modName]; ok && len(funcSections) > 0 {
			sections = append(sections, schema.Section{
				ID:         "func",
				ShortLabel: "func",
				Label:      "Functions",
				SearchKey:  []string{},
				Category:   true,
				Children:   functionsByMod[modName],
			})
		}

		if classSections, ok := classesByMod[modName]; ok && len(classSections) > 0 {
			sections = append(sections, schema.Section{
				ID:         "class",
				ShortLabel: "class",
				Label:      "Classes",
				SearchKey:  []string{},
				Category:   true,
				Children:   classesByMod[modName],
			})
		}

		if len(sections) > 0 {
			pages = append(pages, schema.Page{
				Path:      moduleInfo.path,
				Title:     "Module " + modName,
				Detail:    schema.Markdown(moduleInfo.docs),
				SearchKey: []string{modName},
				Sections:  sections,
			})
		}
	}

	return &schema.Index{
		SchemaVersion: schema.LatestVersion,
		Language:      schema.LanguageJavaScript,
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

func getFunctions(node *sitter.Node, content []byte, q string, searchKeyPrefix []string) ([]schema.Section, error) {
	var functions []schema.Section
	query, err := sitter.NewQuery([]byte(q), javascript.GetLanguage())
	if err != nil {
		return nil, errors.Wrap(err, "NewQuery")
	}
	defer query.Close()

	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, node)

	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		captures := getCaptures(query, match)
		funcDocs := joinCaptures(content, captures["func_docs"], "\n")
		funcDocs = extractFunctionDocs(funcDocs)
		funcName := firstCaptureContentOr(content, captures["func_name"], "")
		funcParams := firstCaptureContentOr(content, captures["func_params"], "")
		funcIdentifier := firstCaptureContentOr(content, captures["var_identifier"], "")
		isArrowFunction := firstCaptureContentOr(content, captures["arrow_function"], "")
		funcLabel := schema.Markdown("function " + funcName + funcParams)
		if funcIdentifier != "" {
			funcName = funcIdentifier
			if isArrowFunction != "" {
				funcLabel = schema.Markdown(funcName + " = " + funcParams)
			} else {
				funcLabel = schema.Markdown(funcName + " =  function " + funcParams)
			}
		}
		functions = append(functions, schema.Section{
			ID:         funcName,
			ShortLabel: funcName,
			Label:      funcLabel,
			Detail:     schema.Markdown(funcDocs),
			SearchKey:  append(searchKeyPrefix, ".", funcName),
		})
	}

	return functions, nil
}

func extractFunctionDocs(s string) string {
	// JSDoc comments must start with a /**
	// sequence in order to be recognized by the JSDoc parser.
	// Comments beginning with /*, /***, or more than 3 stars are ignored by Jsdoc Parser.
	if strings.HasPrefix(s, "/**") && !strings.HasPrefix(s, "/***") {
		comment := []byte(s)
		funcDocs := ""
		node, err := sitter.ParseCtx(context.Background(), comment, jsdoc.GetLanguage())
		if err != nil {
			return ""
		}
		query, err := sitter.NewQuery([]byte(`
		(document
			(description)? @func_description
			(tag
				(tag_name)? @tag_name
				(type)? @identifier_type
				(identifier)? @identifier_name
				(description)? @identifier_description
			)
		)*
		`), jsdoc.GetLanguage())
		if err != nil {
			return ""
		}

		defer query.Close()

		cursor := sitter.NewQueryCursor()
		defer cursor.Close()
		cursor.Exec(query, node)

		argsSection := ""
		returnSection := ""
		for {
			match, ok := cursor.NextMatch()
			if !ok {
				break
			}
			captures := getCaptures(query, match)
			funcDescription := firstCaptureContentOr(comment, captures["func_description"], "")
			funcDocs += fmt.Sprintf("%s\n", funcDescription)

			identifierType := firstCaptureContentOr(comment, captures["identifier_type"], "")
			if identifierType != "" {
				identifierType = fmt.Sprintf(" (%s)", identifierType)
			}
			identifierName := firstCaptureContentOr(comment, captures["identifier_name"], "")
			identifierDescription := firstCaptureContentOr(comment, captures["identifier_description"], "")
			if identifierDescription != "" {
				identifierDescription = fmt.Sprintf(": %s", identifierDescription)
			}

			tagName := firstCaptureContentOr(comment, captures["tag_name"], "")
			switch tagName {
			case "@param":
				argsSection += fmt.Sprintf("\n\t%s%s%s", identifierName, identifierType, identifierDescription)
			case "@return":
				returnSection += fmt.Sprintf("\n\t%s%s%s", identifierName, identifierType, identifierDescription)
			}
		}

		if len(argsSection) > 0 {
			funcDocs += fmt.Sprintf("\n Arguments:\n%s", argsSection)
		}

		if len(returnSection) > 0 {
			funcDocs += fmt.Sprintf("\n Returns:\n%s", returnSection)
		}

		return funcDocs
	}

	return sanitizeDocs(s)
}

func sanitizeDocs(s string) string {
	if strings.HasPrefix(s, "//") {
		s = strings.ReplaceAll(s, "\n//", "\n")
		return strings.TrimPrefix(s, "//")
	} else if strings.HasPrefix(s, "/*") {
		return strings.TrimSuffix(strings.TrimPrefix(s, "/*"), "*/")
	}
	return s
}

func functionDefinitionQuery() string {
	functionDefinition := `(
		function
			name: (identifier)? @func_name
			parameters: (formal_parameters) @func_params
	) `

	arrowFunctionDefinition := `(
		arrow_function
			parameters: (formal_parameters) @func_params
	) @arrow_function`

	// function myfunc(){}
	funcDeclaration := ` 			
	(
		(comment)* @func_docs
		.
		(
			function_declaration
				name: (identifier) @func_name
				parameters: (formal_parameters) @func_params
		)
	)
	`
	// var myfunction = function(a,b){}
	// var myfunction = (a,b) => {}
	funcAssignmentExpression := fmt.Sprintf(`(
		(comment)* @func_docs
		.
		(lexical_declaration
			(_
				name: (identifier) @var_identifier
				value: [
					%s
					%s
				]
				
			
			)
		)
	)`, functionDefinition, arrowFunctionDefinition)

	// export default myfunc = function(){}
	// export default myfunc = () => {}
	funcExportExpression := fmt.Sprintf(`(
		(comment)* @func_docs
		.
		(export_statement
			(lexical_declaration
				(_
					name: (identifier) @var_identifier
					value: [
						%s
						%s
					]
								
				)
			)?
			value:([
				%s
				%s
			])?
		)
		
	)`, functionDefinition, arrowFunctionDefinition, functionDefinition, arrowFunctionDefinition)

	// module.exports = function(){}
	funcExpressionStatementAssignment := fmt.Sprintf(`
	(
		(comment)* @func_docs
		.
		(expression_statement
			(assignment_expression
				left: (_) @var_identifier
				right: 
				[
					%s
					%s
				]			
			)
		)
	)`, functionDefinition, arrowFunctionDefinition)

	query := fmt.Sprintf(`
	(
		[
			%s
			%s
			%s
			%s	
		]
	 )
	`, funcDeclaration, funcAssignmentExpression, funcExportExpression, funcExpressionStatementAssignment)
	return query
}

type moduleInfo struct {
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

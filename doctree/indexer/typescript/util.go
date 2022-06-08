package typescript

import (
	"bytes"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

func treeToString(input []byte, node *sitter.Node) string {
	var buf bytes.Buffer
	printTree(&buf, input, node, "", "")
	return buf.String()
}

func printTree(buf *bytes.Buffer, input []byte, node *sitter.Node, indent, fieldName string) {
	fieldString := ""
	if fieldName != "" {
		fieldString = fmt.Sprintf("[%s] ", fieldName)
	}
	if node.Type() == "identifier" {
		fmt.Fprintf(buf, "%s%s%s %s %v\n", indent, fieldString, node.Content(input), node.Type(), node.IsNamed())
	} else {
		fmt.Fprintf(buf, "%s%s%s %v\n", indent, fieldString, node.Type(), node.IsNamed())
	}

	for _, cf := range getChildFields(node) {
		printTree(buf, input, cf.Child, indent+"  ", cf.Field)
	}
}

type ChildField struct {
	Field string
	Child *sitter.Node
}

func getChildFields(node *sitter.Node) (f []ChildField) {
	cu := sitter.NewTreeCursor(node)
	ok := cu.GoToFirstChild()
	if !ok {
		return
	}
	f = append(f, ChildField{cu.CurrentFieldName(), cu.CurrentNode()})
	for {
		ok := cu.GoToNextSibling()
		if !ok {
			break
		}
		f = append(f, ChildField{cu.CurrentFieldName(), cu.CurrentNode()})
	}
	return
}

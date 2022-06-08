// Package typescript provides a doctree indexer implementation for TypeScript.
package typescript

import (
	"context"

	"github.com/sourcegraph/doctree/doctree/indexer"
	"github.com/sourcegraph/doctree/doctree/schema"
)

func init() {
	indexer.Register(&typescriptIndexer{})
}

// Implements the indexer.Language interface.
type typescriptIndexer struct{}

func (i *typescriptIndexer) Name() schema.Language { return schema.LanguageTypeScript }

func (i *typescriptIndexer) Extensions() []string { return []string{"js"} }

func (i *typescriptIndexer) IndexDir(ctx context.Context, dir string) (*schema.Index, error) {
	return nil, nil
}

// Package apischema defines the JSON types that are returned by the doctree JSON API.
package apischema

import "github.com/sourcegraph/doctree/doctree/schema"

// ProjectList is the type returned by /api/list
type ProjectList []string

// ProjectIndexes is the type returned by /api/get?name=github.com/sourcegraph/sourcegraph
type ProjectIndexes map[string]schema.Index

// SearchResult is the type returned by /api/search?query=foobar
type SearchResult struct {
	Language    string  `json:"language"`
	ProjectName string  `json:"projectName"`
	SearchKey   string  `json:"searchKey"`
	Path        string  `json:"path"`
	ID          string  `json:"id"`
	Score       float64 `json:"score"`
}

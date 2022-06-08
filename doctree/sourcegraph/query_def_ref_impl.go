package sourcegraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

func defRefImplQuery(n int) string {
	var aliased []string
	var params []string
	for i := 0; i < n; i++ {
		params = append(params, fmt.Sprintf("$line%v: Int!, $character%v: Int!", i, i))
		aliased = append(aliased, fmt.Sprintf(`
							references%v: references(
								line: $line%v
								character: $character%v
								first: $firstReferences
								after: $afterReferences
								filter: $filter
							) {
								...LocationConnectionFields
							}
							implementations%v: implementations(
								line: $line%v
								character: $character%v
								first: $firstImplementations
								after: $afterImplementations
								filter: $filter
							) {
								...LocationConnectionFields
							}
							definitions%v: definitions(line: $line%v, character: $character%v, filter: $filter) {
								...LocationConnectionFields
							}
		`, i, i, i, i, i, i, i, i, i))
	}

	return fmt.Sprintf(`
		fragment LocationConnectionFields on LocationConnection {
			nodes {
				url
				resource {
					path
					content
					repository {
						name
					}
					commit {
						oid
					}
				}
				range {
					start {
						line
						character
					}
					end {
						line
						character
					}
				}
			}
			pageInfo {
				endCursor
			}
		}

		query UsePreciseCodeIntelForPosition($repositoryCloneUrl: String!, $commit: String!, $path: String!, $afterReferences: String, $firstReferences: Int, $afterImplementations: String, $firstImplementations: Int, $filter: String, %s) {
			repository(cloneURL: $repositoryCloneUrl) {
				id
				name
				stars
				isFork
				isArchived
				commit(rev: $commit) {
					id
					blob(path: $path) {
						lsif {
							%s
						}
					}
				}
			}
		}
	`, strings.Join(params, ", "), strings.Join(aliased, "\n"))
}

type Position struct {
	Line      uint `json:"line"`
	Character uint `json:"character"`
}

type DefRefImplArgs struct {
	AfterImplementations *string `json:"afterImplementations"`
	AfterReferences      *string `json:"afterReferences"`
	RepositoryCloneURL   string  `json:"repositoryCloneUrl"`
	Path                 string  `json:"path"`
	Positions            []Position
	Commit               string  `json:"commit"`
	Filter               *string `json:"filter"`
	FirstImplementations uint    `json:"firstImplementations"`
	FirstReferences      uint    `json:"firstReferences"`
}

func (c *graphQLClient) DefRefImpl(ctx context.Context, args DefRefImplArgs) (*Repository, error) {
	vars := map[string]interface{}{
		"afterImplementations": args.AfterImplementations,
		"afterReferences":      args.AfterReferences,
		"repositoryCloneUrl":   args.RepositoryCloneURL,
		"path":                 args.Path,
		"commit":               args.Commit,
		"filter":               args.Filter,
		"firstImplementations": args.FirstImplementations,
		"firstReferences":      args.FirstReferences,
	}
	for i, pos := range args.Positions {
		vars[fmt.Sprintf("line%v", i)] = pos.Line
		vars[fmt.Sprintf("character%v", i)] = pos.Character
	}

	resp, err := c.requestGraphQL(ctx, "DefRefImpl", defRefImplQuery(len(args.Positions)), vars)
	if err != nil {
		return nil, errors.Wrap(err, "graphql")
	}
	var raw struct {
		Data struct {
			Repository DefRefImplRepository
		}
	}
	if err := json.Unmarshal(resp, &raw); err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	var (
		r               = raw.Data.Repository
		references      []Location
		implementations []Location
		definitions     []Location
	)
	decodeLocation := func(name string, dst *[]Location) error {
		raw, ok := r.Commit.Blob.LSIF[name]
		if !ok {
			return nil
		}
		var result *Location
		if err := json.Unmarshal(raw, &result); err != nil {
			return errors.Wrap(err, "Unmarshal")
		}
		if result != nil {
			*dst = append(*dst, *result)
		}
		return nil
	}
	for i := range args.Positions {
		if err := decodeLocation(fmt.Sprintf("references%v", i), &references); err != nil {
			return nil, errors.Wrap(err, "decodeLocation(references)")
		}
		if err := decodeLocation(fmt.Sprintf("implementations%v", i), &implementations); err != nil {
			return nil, errors.Wrap(err, "decodeLocation(implementations)")
		}
		if err := decodeLocation(fmt.Sprintf("definitions%v", i), &definitions); err != nil {
			return nil, errors.Wrap(err, "decodeLocation(definitions)")
		}
	}
	return &Repository{
		ID:         r.ID,
		Name:       r.Name,
		Stars:      r.Stars,
		IsFork:     r.IsFork,
		IsArchived: r.IsArchived,
		Commit: &Commit{
			ID:  r.Commit.ID,
			OID: r.Commit.OID,
			Blob: &Blob{
				LSIF: &LSIFBlob{
					References:      references,
					Implementations: implementations,
					Definitions:     definitions,
				},
			},
		},
	}, nil
}

type DefRefImplBlob struct {
	LSIF map[string]json.RawMessage
}

type DefRefImplCommit struct {
	ID   string
	OID  string
	Blob *DefRefImplBlob
}

type DefRefImplRepository struct {
	ID         string
	Name       string
	Stars      uint64
	IsFork     bool
	IsArchived bool
	Commit     *DefRefImplCommit `json:"commit"`
}

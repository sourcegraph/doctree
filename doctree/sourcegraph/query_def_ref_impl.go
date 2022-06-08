package sourcegraph

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
)

const defRefImplQuery = `
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

	query UsePreciseCodeIntelForPosition($repositoryCloneUrl: String!, $commit: String!, $path: String!, $line: Int!, $character: Int!, $afterReferences: String, $firstReferences: Int, $afterImplementations: String, $firstImplementations: Int, $filter: String) {
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
						references(
							line: $line
							character: $character
							first: $firstReferences
							after: $afterReferences
							filter: $filter
						) {
							...LocationConnectionFields
						}
						implementations(
							line: $line
							character: $character
							first: $firstImplementations
							after: $afterImplementations
							filter: $filter
						) {
							...LocationConnectionFields
						}
						definitions(line: $line, character: $character, filter: $filter) {
							...LocationConnectionFields
						}
					}
				}
			}
		}
	}
`

type DefRefImplArgs struct {
	AfterImplementations *string `json:"afterImplementations"`
	AfterReferences      *string `json:"afterReferences"`
	RepositoryCloneURL   string  `json:"repositoryCloneUrl"`
	Path                 string  `json:"path"`
	Line                 uint    `json:"line"`
	Character            uint    `json:"character"`
	Commit               string  `json:"commit"`
	Filter               *string `json:"filter"`
	FirstImplementations uint    `json:"firstImplementations"`
	FirstReferences      uint    `json:"firstReferences"`
}

func (c *graphQLClient) DefRefImpl(ctx context.Context, args DefRefImplArgs) (*Repository, error) {
	resp, err := c.requestGraphQL(ctx, "DefRefImpl", defRefImplQuery, args)
	if err != nil {
		return nil, errors.Wrap(err, "graphql")
	}
	var result struct {
		Data struct {
			Repository Repository
		}
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return &result.Data.Repository, nil
}

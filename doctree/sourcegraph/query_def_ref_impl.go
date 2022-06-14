package sourcegraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

func defRefImplQuery(args DefRefImplArgs) string {
	var aliased []string
	var params []string

	for fileIndex, file := range args.Files {
		params = append(params, fmt.Sprintf("$file%vpath: String!", fileIndex))
		var aliasedFile []string
		for i := range file.Positions {
			params = append(params, fmt.Sprintf("$file%vline%v: Int!", fileIndex, i))
			params = append(params, fmt.Sprintf("$file%vcharacter%v: Int!", fileIndex, i))
			aliasedFile = append(aliasedFile, strings.ReplaceAll(strings.ReplaceAll(`
							references#P: references(
								line: $file#Fline#P
								character: $file#Fcharacter#P
								first: $firstReferences
								after: $afterReferences
								filter: $filter
							) {
								...LocationConnectionFields
							}
							# TODO: query implementations once that API does not take down prod:
							# https://github.com/sourcegraph/sourcegraph/issues/36882
							#implementations#P: implementations(
							#	line: $file#Fline#P
							#	character: $file#Fcharacter#P
							#	first: $firstImplementations
							#	after: $afterImplementations
							#	filter: $filter
							#) {
							#	...LocationConnectionFields
							#}
							definitions#P: definitions(line: $file#Fline#P, character: $file#Fcharacter#P, filter: $filter) {
								...LocationConnectionFields
							}
			`, "#F", fmt.Sprint(fileIndex)), "#P", fmt.Sprint(i)))
		}
		aliased = append(aliased, fmt.Sprintf(`
					blob%v: blob(path: $file%vpath) {
						lsif {
							%s
						}
					}
		`, fileIndex, fileIndex, strings.Join(aliasedFile, "\n")))
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

		query UsePreciseCodeIntelForPosition(
			$repositoryCloneUrl: String!,
			$commit: String!,
			$afterReferences: String,
			$firstReferences: Int,
			# TODO: query implementations once that API does not take down prod:
			# https://github.com/sourcegraph/sourcegraph/issues/36882
			#$afterImplementations: String,
			#$firstImplementations: Int,
			$filter: String, %s) {
			repository(cloneURL: $repositoryCloneUrl) {
				id
				name
				stars
				isFork
				isArchived
				commit(rev: $commit) {
					id
					%s
				}
			}
		}
	`, strings.Join(params, ", "), strings.Join(aliased, "\n"))
}

type Position struct {
	Line      uint `json:"line"`
	Character uint `json:"character"`
}

type File struct {
	Path      string `json:"path"`
	Positions []Position
}

type DefRefImplArgs struct {
	AfterImplementations *string `json:"afterImplementations"`
	AfterReferences      *string `json:"afterReferences"`
	RepositoryCloneURL   string  `json:"repositoryCloneUrl"`
	Files                []File
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
		"commit":               args.Commit,
		"filter":               args.Filter,
		"firstImplementations": args.FirstImplementations,
		"firstReferences":      args.FirstReferences,
	}
	for fileIndex, file := range args.Files {
		vars[fmt.Sprintf("file%vpath", fileIndex)] = file.Path
		for i, pos := range file.Positions {
			vars[fmt.Sprintf("file%vline%v", fileIndex, i)] = pos.Line
			vars[fmt.Sprintf("file%vcharacter%v", fileIndex, i)] = pos.Character
		}
	}

	resp, err := c.requestGraphQL(ctx, "DefRefImpl", defRefImplQuery(args), vars)
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
		r     = raw.Data.Repository
		blobs []Blob
	)
	for fileIndex, file := range args.Files {
		rawBlob, ok := r.Commit[fmt.Sprintf("blob%v", fileIndex)]
		if !ok {
			continue
		}
		var info struct {
			LSIF map[string]json.RawMessage
		}
		if err := json.Unmarshal(rawBlob, &info); err != nil {
			return nil, errors.Wrap(err, "Unmarshal")
		}

		decodeLocation := func(name string, dst *[]Location) error {
			raw, ok := info.LSIF[name]
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
		blob := Blob{LSIF: &LSIFBlob{}}
		for i := range file.Positions {
			if err := decodeLocation(fmt.Sprintf("references%v", i), &blob.LSIF.References); err != nil {
				return nil, errors.Wrap(err, "decodeLocation(references)")
			}
			// TODO: query implementations once that API does not take down prod:
			// https://github.com/sourcegraph/sourcegraph/issues/36882
			// if err := decodeLocation(fmt.Sprintf("implementations%v", i), &blob.LSIF.Implementations); err != nil {
			// 	return nil, errors.Wrap(err, "decodeLocation(implementations)")
			// }
			if err := decodeLocation(fmt.Sprintf("definitions%v", i), &blob.LSIF.Definitions); err != nil {
				return nil, errors.Wrap(err, "decodeLocation(definitions)")
			}
		}
		blobs = append(blobs, blob)
	}
	var commitID string
	_ = json.Unmarshal(r.Commit["id"], &commitID)
	var commitOID string
	_ = json.Unmarshal(r.Commit["oid"], &commitOID)
	return &Repository{
		ID:         r.ID,
		Name:       r.Name,
		Stars:      r.Stars,
		IsFork:     r.IsFork,
		IsArchived: r.IsArchived,
		Commit: &Commit{
			ID:    commitID,
			OID:   commitOID,
			Blobs: blobs,
		},
	}, nil
}

type DefRefImplRepository struct {
	ID         string
	Name       string
	Stars      uint64
	IsFork     bool
	IsArchived bool
	Commit     map[string]json.RawMessage
}

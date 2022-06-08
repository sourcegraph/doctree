// Package sourcegraph provides the Sourcegraph API.
package sourcegraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

// graphQLQuery describes a general GraphQL query and its variables.
type graphQLQuery struct {
	Query     string `json:"query"`
	Variables any    `json:"variables"`
}

type graphQLClient struct {
	opt    Options
	client *http.Client
}

// requestGraphQL performs a GraphQL request with the given query and variables.
// search executes the given search query. The queryName is used as the source of the request.
// The result will be decoded into the given pointer.
func (c *graphQLClient) requestGraphQL(ctx context.Context, queryName string, query string, variables any) ([]byte, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(graphQLQuery{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Encode")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.opt.URL+"/.api/graphql?doctree"+queryName, &buf)
	if err != nil {
		return nil, errors.Wrap(err, "Post")
	}

	if c.opt.Token != "" {
		req.Header.Set("Authorization", "token "+c.opt.Token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Post")
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "ReadAll")
	}

	var errs struct {
		Errors []any
	}
	if err := json.Unmarshal(data, &errs); err != nil {
		return nil, errors.Wrap(err, "Unmarshal errors")
	}
	if len(errs.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %v", errs.Errors)
	}
	return data, nil
}

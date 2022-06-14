// Package sourcegraph provides a Sourcegraph API client.
package sourcegraph

import (
	"context"
	"net"
	"net/http"
	"time"
)

// Options describes client options.
type Options struct {
	// URL is the Sourcegraph instance URL, e.g. "https://sourcegraph.com"
	URL string

	// Token is a Sourcegraph API access token, or an empty string.
	Token string
}

type Client interface {
	DefRefImpl(context.Context, DefRefImplArgs) (*Repository, error)
}

func New(opt Options) Client {
	tr := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 3*time.Second)
		},
		ResponseHeaderTimeout: 0,
	}
	return &graphQLClient{
		opt: opt,
		client: &http.Client{
			Transport: tr,
			Timeout:   60 * time.Second,
		},
	}
}

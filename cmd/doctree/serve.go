package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"

	"github.com/NYTimes/gziphandler"
	"github.com/hexops/cmder"
	"github.com/pkg/errors"
	"github.com/sourcegraph/doctree/doctree/indexer"
	"github.com/sourcegraph/doctree/frontend"
)

func init() {
	const usage = `
Examples:

  Start a doctree server:

    $ doctree serve

  Use a specific port:

    $ doctree serve -http=:3333

`

	// Parse flags for our subcommand.
	flagSet := flag.NewFlagSet("serve", flag.ExitOnError)
	dataDirFlag := flagSet.String("data-dir", defaultDataDir(), "where doctree stores its data")
	httpFlag := flagSet.String("http", ":3333", "address to bind for the HTTP server")

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		indexDataDir := filepath.Join(*dataDirFlag, "index")
		return Serve(*httpFlag, indexDataDir)
	}

	// Register the command.
	commands = append(commands, &cmder.Command{
		FlagSet: flagSet,
		Aliases: []string{"server"},
		Handler: handler,
		UsageFunc: func() {
			fmt.Fprintf(flag.CommandLine.Output(), "Usage of 'doctree %s':\n", flagSet.Name())
			flagSet.PrintDefaults()
			fmt.Fprintf(flag.CommandLine.Output(), "%s", usage)
		},
	})
}

// Serve an HTTP server on the given addr.
func Serve(addr, indexDataDir string) error {
	log.Printf("Listening on %s", addr)
	mux := http.NewServeMux()
	mux.Handle("/", frontendHandler())
	mux.Handle("/api/list", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SECURITY: This endpoint isn't mutable and doesn't serve privileged information, and
		// therefor safe to use from any origin.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		indexes, err := indexer.List(indexDataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		b, err := json.Marshal(indexes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(b)
		if err != nil {
			return
		}
	}))
	mux.Handle("/api/get", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SECURITY: This endpoint isn't mutable and doesn't serve privileged information, and
		// therefor safe to use from any origin.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		projectName := r.URL.Query().Get("name")
		projectIndexes, err := indexer.Get(indexDataDir, projectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		b, err := json.Marshal(projectIndexes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(b)
		if err != nil {
			return
		}
	}))
	muxWithGzip := gziphandler.GzipHandler(mux)
	if err := http.ListenAndServe(addr, muxWithGzip); err != nil {
		return errors.Wrap(err, "ListenAndServe")
	}
	return nil
}

func frontendHandler() http.Handler {
	if debugServer := os.Getenv("ELM_DEBUG_SERVER"); debugServer != "" {
		// Reverse proxy to the elm-spa debug server for hot code reloading, etc.
		remote, err := url.Parse(debugServer)
		if err != nil {
			panic(err)
		}
		return httputil.NewSingleHostReverseProxy(remote)
	}

	// Server assets that are embedded into Go binary.
	return http.FileServer(http.FS(frontend.EmbeddedFS()))
}

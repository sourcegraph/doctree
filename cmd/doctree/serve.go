package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/NYTimes/gziphandler"
	"github.com/fsnotify/fsnotify"
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
	cloudModeFlag := flagSet.Bool("cloud", false, "run in cloud mode (i.e. doctree.org)")

	// Handles calls to our subcommand.
	handler := func(args []string) error {
		_ = flagSet.Parse(args)
		indexDataDir := filepath.Join(*dataDirFlag, "index")

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

		go ListenAutoIndexedProjects(dataDirFlag)
		go Serve(*cloudModeFlag, *httpFlag, indexDataDir)
		<-signals

		return nil
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
func Serve(cloudMode bool, addr, indexDataDir string) {
	log.Printf("Listening on %s", addr)
	mux := http.NewServeMux()
	mux.Handle("/", frontendHandler())
	mux.Handle("/main.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flags := struct {
			CloudMode bool `json:"cloudMode"`
		}{CloudMode: cloudMode}

		flagsJson, err := json.Marshal(flags)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintf(w, `Elm.Main.init({flags: %s})`, flagsJson)
	}))
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
	mux.Handle("/api/search", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SECURITY: This endpoint isn't mutable and doesn't serve privileged information, and
		// therefor safe to use from any origin.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		query := r.URL.Query().Get("query")
		results, err := indexer.Search(r.Context(), indexDataDir, query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		b, err := json.Marshal(results)
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
		log.Fatal(errors.Wrap(err, "ListenAndServe"))
	}
}

func frontendHandler() http.Handler {
	if debugServer := os.Getenv("ELM_DEBUG_SERVER"); debugServer != "" {
		// Reverse proxy to the elm-spa debug server for hot code reloading, etc.
		remote, err := url.Parse(debugServer)
		if err != nil {
			panic(err)
		}
		proxy := httputil.NewSingleHostReverseProxy(remote)

		// Dev server hack to fix requests for "/github.com" etc. that appear as a request for file
		// due to extension (.com), see public/index.html for more info.
		defaultDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			defaultDirector(req)
			_, err := os.Stat(filepath.Join("frontend/public", req.URL.Path))
			if os.IsNotExist(err) {
				queryParams := req.URL.RawQuery

				req.URL.RawQuery = req.URL.Path + "&" + queryParams
				req.URL.Path = "/"
			}
		}
		return proxy
	}

	// Serve assets that are embedded into Go binary.
	fs := http.FS(frontend.EmbeddedFS())
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// If there is not a file present, then this request is likely for a page like
		// "/github.com/sourcegraph/sourcegraph" and we should still serve the SPA. Change the
		// request path to "/" prior to serving so index.html is what gets served.
		f, err := fs.Open(req.URL.Path)
		if err != nil {
			req.URL.Path = "/"
		} else {
			f.Close()
		}

		fileServer.ServeHTTP(w, req)
	})
}

func isParentDir(parent, child string) (bool, error) {
	relativePath, err := filepath.Rel(parent, child)
	if err != nil {
		return false, err
	}
	return !strings.Contains(relativePath, ".."), nil
}

func ListenAutoIndexedProjects(dataDirFlag *string) {
	// Read the list of projects to monitor.
	autoIndexPath := filepath.Join(*dataDirFlag, "autoindex")
	autoindexedProjects, err := ReadAutoIndex(autoIndexPath)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the fsnotify watcher
	// TODO: Watch ~/.doctree/autoindex
	// to re-index newly added projects on the fly?
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Configure watcher to watch all dirs mentioned in the 'autoindex' file
	for i, project := range autoindexedProjects {
		if GetDirHash(project.Path) != project.Hash {
			log.Printf("Project %s has been modified while server was down, reindexing", project.Name)
			ctx := context.Background()
			if err != nil {
				log.Fatal(err)
			}
			err := RunIndexers(ctx, project.Path, dataDirFlag, &project.Name)
			if err != nil {
				log.Fatal(err)
			}
			// Update the autoIndexedProjects array
			autoindexProjectPtr := &autoindexedProjects[i]
			autoindexProjectPtr.Hash = GetDirHash(project.Path)
			err = WriteAutoIndex(autoIndexPath, autoindexedProjects)
			if err != nil {
				log.Fatal(err)
			}
		}

		// Add the project directory to the watcher
		// TODO: Watch nested directories
		err = watcher.Add(project.Path)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Watching", project)
	}

	f, err := os.Create(autoIndexPath)
	if err != nil {
		log.Fatal(errors.Wrap(err, "Create"))
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(autoindexedProjects); err != nil {
		log.Fatal(errors.Wrap(err, "Encode"))
	}

	done := make(chan error)

	// Process events
	go func() {
		for {
			select {
			case ev := <-watcher.Events:
				log.Println("Event:", ev)
				for _, dir := range autoindexedProjects {
					isParent, err := isParentDir(dir.Path, ev.Name)
					if err != nil {
						log.Println(err)
						return
					}
					if isParent {
						log.Println("Reindexing", dir)
						ctx := context.Background()
						if err != nil {
							log.Println(err)
							return
						}
						err := RunIndexers(ctx, dir.Path, dataDirFlag, &dir.Name)
						if err != nil {
							log.Fatal(err)
						}
						break // Only reindex for the first matching parent
					}
				}
			case err := <-watcher.Errors:
				log.Println("Error:", err)
			}
		}
	}()
	<-done
}

package main

import (
	"bytes"
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
	"strconv"
	"syscall"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/fsnotify/fsnotify"
	"github.com/hexops/cmder"
	"github.com/pkg/errors"
	"github.com/sourcegraph/doctree/doctree/apischema"
	"github.com/sourcegraph/doctree/doctree/indexer"
	"github.com/sourcegraph/doctree/doctree/schema"
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

		go Serve(*cloudModeFlag, *httpFlag, *dataDirFlag, indexDataDir)
		go func() {
			err := ListenAutoIndexedProjects(dataDirFlag)
			if err != nil {
				log.Fatal(err)
			}
		}()
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
func Serve(cloudMode bool, addr, dataDir, indexDataDir string) {
	log.Printf("Listening on %s", addr)
	mux := http.NewServeMux()
	mux.Handle("/", frontendHandler(cloudMode))
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
		fmt.Fprintf(w, `
let app = Elm.Main.init({flags: %s});

let onObserved = (entries, observer) => {
	if (app.ports.onObserved) {
		app.ports.onObserved.send(entries.map(entry => {
			let rootCenter = entry.rootBounds.y + (entry.rootBounds.height/2.0);
			let entryCenter = entry.intersectionRect.y + (entry.intersectionRect.height/2.0);
			let distanceToCenter = Math.abs(rootCenter - entryCenter);
			// NOTE: distanceToCenter only updates, i.e. events are only sent, when threshold
			// changes. So if the element is completely in view, distanceToCenter will not update.
			return ({
				isIntersecting: entry.isIntersecting,
				intersectionRatio: entry.intersectionRatio,
				distanceToCenter: distanceToCenter,
				targetID: entry.target.id
			});
		}));
	}
};

let observer = new IntersectionObserver(onObserved, {
	threshold: 0, // 1px
	// intersect when an element crosses the horizontal line at center of screen.
	rootMargin: '-50%% 0%% -50%% 0%%',
});

app.ports.observeElementID.subscribe(function(id) {
	requestAnimationFrame(() => {
		let target = document.getElementById(id);
		if (!target) {
			console.error("warning: observeElementID given invalid ID: " + id);
			return;
		}
		observer.observe(target);
	});
});
`, flagsJson)
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
	mux.Handle("/api/get-page", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SECURITY: This endpoint isn't mutable and doesn't serve privileged information, and
		// therefor safe to use from any origin.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		projectName := r.URL.Query().Get("project")
		language := r.URL.Query().Get("language")
		pagePath := r.URL.Query().Get("page")

		projectIndexes, err := indexer.GetIndex(r.Context(), dataDir, indexDataDir, projectName, cloudMode)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Lookup the page in the index.
		index, ok := projectIndexes[language]
		if !ok {
			http.Error(w, "no such language for this project", http.StatusNotFound)
			return
		}
		// TODO: require library from client?
		var found *schema.Page
		for _, lib := range index.Libraries {
			for _, page := range lib.Pages {
				if page.Path == pagePath {
					found = &page
					break
				}
				for _, subPage := range page.Subpages {
					if subPage.Path == pagePath {
						found = &page
						break
					}
				}
				if found != nil {
					break
				}
			}
		}
		if found == nil {
			http.Error(w, "page not found", http.StatusNotFound)
			return
		}

		b, err := json.Marshal(apischema.Page(*found))
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

		projectIndexes, err := indexer.GetIndex(r.Context(), dataDir, indexDataDir, projectName, cloudMode)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Eliminate detailed information from the project indexes. This basically just leaves a list
		// of all libraries and pages in the project, with metadata about them - but not the actual
		// content of the pages themselves.
		//
		// Reduces the download size of e.g. golang/go from 4/37 MiB to just 10/81 KiB (compressed/uncompressed)
		cpy := make(apischema.ProjectIndexes, len(projectIndexes))
		for lang, index := range projectIndexes {
			indexCpy := index
			indexCpy.Libraries = make([]schema.Library, 0, len(index.Libraries))
			for _, lib := range index.Libraries {
				libCpy := lib
				libCpy.Pages = make([]schema.Page, 0, len(lib.Pages))
				for _, page := range lib.Pages {
					pageCpy := page
					pageCpy.Sections = []schema.Section{}
					pageCpy.Detail = ""
					pageCpy.Subpages = make([]schema.Page, 0, len(page.Subpages))
					for _, subPage := range page.Subpages {
						subPageCpy := subPage
						subPageCpy.Sections = []schema.Section{}
						subPageCpy.Detail = ""
						pageCpy.Subpages = append(pageCpy.Subpages, subPageCpy)
					}
					libCpy.Pages = append(libCpy.Pages, pageCpy)
				}
				indexCpy.Libraries = append(indexCpy.Libraries, libCpy)
			}
			cpy[lang] = indexCpy
		}
		b, err := json.Marshal(cpy)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(b)
		if err != nil {
			return
		}
	}))
	mux.Handle("/api/get-index", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SECURITY: This endpoint isn't mutable and doesn't serve privileged information, and
		// therefor safe to use from any origin.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		projectName := r.URL.Query().Get("name")

		projectIndexes, err := indexer.GetIndex(r.Context(), dataDir, indexDataDir, projectName, cloudMode)
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
		projectName := r.URL.Query().Get("project")

		// Whether or not this is an autocomplete query. If so, the goal is to find results quickly
		// enough that they are as-you-type. Otherwise, the intent is to really search - and maybe
		// we can do a bit more work to find better results.
		autoComplete, _ := strconv.ParseBool(r.URL.Query().Get("autocomplete"))

		start := time.Now()
		results, err := indexer.Search(r.Context(), indexDataDir, query, projectName)
		duration := time.Since(start)

		// If this is not an autocomplete query (ran very quickly), but rather an intentful one
		// (e.g. the user stopped typing for a full second) then it's a good candidate for logging.
		//
		// This is the only way we have to know if people are getting results they want, quickly,
		// and if we could make the query syntax any nicer / more obvious. The logs are reviewed by
		// human, it's only turned on for doctree.org, and there is no username/IP/etc associated at
		// all.
		if cloudMode && !autoComplete {
			logProject := projectName
			if logProject == "" {
				logProject = "all"
			}
			if err != nil {
				fmt.Printf("search: query error: %v after %vms (project: %s): %s\n", err, duration.Milliseconds(), logProject, query)
			} else {
				var buf bytes.Buffer
				fmt.Fprintf(&buf, "search: found %v results in %vms (project: %s):\n", len(results), duration.Milliseconds(), logProject)
				fmt.Fprintf(&buf, "\tquery: %s\n", query)
				max5 := results
				if len(max5) > 5 {
					max5 = max5[:5]
				}
				for i, result := range max5 {
					fmt.Fprintf(&buf, "\t%d. %s %s\n", i,
						result.SearchKey,
						result.ProjectName,
					)
					fmt.Fprintf(&buf, "\t\tlanguage=%s path=%q id=%q score=%f\n",
						result.Language,
						result.Path,
						result.ID,
						result.Score,
					)
				}
				fmt.Printf("%s", buf.String())
			}
		}

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

func frontendHandler(cloudMode bool) http.Handler {
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

			if cloudMode && req.URL.Path == "/" {
				req.URL.Path = "/index-cloud.html"
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

		if cloudMode && req.URL.Path == "/" {
			req.URL.Path = "/index-cloud.html"
		}

		fileServer.ServeHTTP(w, req)
	})
}

func ListenAutoIndexedProjects(dataDirFlag *string) error {
	// Read the list of projects to monitor.
	autoIndexPath := filepath.Join(*dataDirFlag, "autoindex")
	autoindexedProjects, err := indexer.ReadAutoIndex(autoIndexPath)
	if err != nil {
		return err
	}

	// Initialize the fsnotify watcher
	// TODO: Watch ~/.doctree/autoindex to re-index newly added projects on the fly?
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Configure watcher to watch all dirs mentioned in the 'autoindex' file
	for projectPath := range autoindexedProjects {
		// Add the project directory to the watcher
		// TODO: Check if the project changed while the server wasn't running.
		err = recursiveWatch(watcher, projectPath)
		if err != nil {
			return err
		}
		log.Println("Watching", projectPath)
	}

	err = indexer.WriteAutoIndex(autoIndexPath, autoindexedProjects)
	if err != nil {
		return err
	}
	done := make(chan error)

	// Process events
	go func() {
		for {
			select {
			case ev := <-watcher.Events:
				log.Println("Event:", ev)
				for projectPath, project := range autoindexedProjects {
					isParent, err := isParentDir(projectPath, ev.Name)
					if err != nil {
						log.Println(err)
						return
					}
					if isParent {
						log.Println("Reindexing", projectPath)
						ctx := context.Background()
						if err != nil {
							log.Println(err)
							return
						}
						err := indexer.RunIndexers(ctx, projectPath, *dataDirFlag, project.Name)
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

	return nil
}

# Development

## Prerequisites

* Go v1.18+
* Zig v0.10+ ([nightly binary releases](https://ziglang.org/download/))
* A recent / latest LTS version of [Node.js](https://nodejs.org/)

[Task](https://taskfile.dev/#/installation) (alternative to `make` with file change watching):

```sh
brew install go-task/tap/go-task
```

Then ensure [Elm](https://elm-lang.org/), [elm-spa](https://elm-spa.dev), frontend dependencies and Zig libraries are cloned using:

```sh
task setup
```

## Suggested tooling

* For developing Elm frontend code, install [the "wrench" icon Elm plugin in VS Code](https://marketplace.visualstudio.com/items?itemName=Elmtooling.elm-ls-vscode)
* For developing Zig code (unlikely), ensure you're using latest Zig version and [build zls from source](https://github.com/zigtools/zls#from-source), then install "ZLS for VSCode".

## Working with the code

Just run `task` in the repository root and navigate to http://localhost:3333 - it'll do everything for you:

* Build development tools for you (Go linter, gofumpt code formatter, etc.)
* `go generate` any necessary code for you
* Run Go linters and gofumpt code formatter for you.
* Build and run `.bin/doctree serve` for you

Best of all, it'll live reload the frontend code as you save changes in your editor. No need to even refresh the page!

## Sample repositories

You can use the following to clone some sample repositories (into `../doctree-samples`) - useful for testing every language supported by Doctree:

```sh
task dev-clone-sample-repos
```

And then use the following to index them all:

```sh
task dev-index-sample-repos
```

## Running tests

You can use `task test` or `task test-race` (slower, but checks for race conditions).

## Building Docker image

`task build-image` will build and tag a `sourcegraph/doctree:dev` image for you. `task run-image` will run it!

## Cross-compiling binaries for each OS

If you have a macOS host machine you should be able to cross-compile binaries for each OS:

```
task cross-compile
```

Which should produce an `out/` directory with binaries for each OS.

If not on macOS, you can use the `task cc-x86_64-linux` and `task cc-x86_64-windows` targets only for now.

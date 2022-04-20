# Development

## Prerequisites

* Go v1.18+
* A recent / latest LTS version of [Node.js](https://nodejs.org/)

[Task](https://taskfile.dev/#/installation) (alternative to `make` with file change watching):

```sh
brew install go-task/tap/go-task
```

[Elm](https://elm-lang.org/) and [elm-spa](https://elm-spa.dev):

```sh
npm install -g elm elm-spa
```

## Working with the code

Just run `task --watch` in the repository root, it'll automatically do everything you need:

* Build development tools for you (linter, code formatter)
* `go generate` any necessary code for you
* Lint and format code for you.
* Build and run `.bin/doctree serve` for you

Best of all, it'll do this as you make changes to the code in your editor!

## Running tests

You can use `task test` or `task test-race` (slower, but checks for race conditions).

## Building Docker image

`task build-image` will build and tag a `sourcegraph/doctree:dev` image for you. `task run-image` will run it!

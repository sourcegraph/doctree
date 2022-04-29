# Experiment! Super early stages!

Imagine an API docsite generator that generates something like http://pkg.go.dev but:

* Works with any language
* Has symbol-level search
* Automatically surfaces usage examples
* Supports both private code and open source

That's what [we're building here!](https://twitter.com/beyang/status/1516563075192680450)

## When can I try it?

If you'd like to try it out once we have an early version, let us know [on Twitter](https://twitter.com/beyang/status/1516563075192680450) or open an issue - we'd love people to try it out as we develop it!

Actual project README begins here :)

---

# doctree: First-class library docs tool for every language

Features (aspirational):

* Symbol search
* Finds usage examples automagically
* Based on tree-sitter
* Runnable standalone or via http://doctree.dev

## Try it out (**EXTREMELY** early stages)

Probably not worth trying out unless you're *incredibly* excited about this idea.

Use Docker for now (working on single-binary builds still):

Run the server:

```sh
docker run -it --publish 3333:3333 --rm --name doctree --volume ~/.doctree:/home/nonroot/.doctree sourcegraph/doctree:dev
```

In a folder with Go code you'd like to see docs for, index it (for a large project like `golang/go` expect it to take ~52s for now. It's not multi-threaded.):

```sh
docker run -it --volume $(pwd):/index --volume ~/.doctree:/home/nonroot/.doctree --entrypoint=sh sourcegraph/doctree:dev -c "cd /index && doctree index ."
```

Navigate to https://localhost:3333

## Screenshots

<img width="976" alt="image" src="https://user-images.githubusercontent.com/3173176/165888825-b9399cb1-7025-4242-9bcd-5773b6382fff.png">

<img width="976" alt="image" src="https://user-images.githubusercontent.com/3173176/165888831-cdc5cd87-7d9c-4465-bf9a-71e019f3f9bb.png">

<img width="1267" alt="image" src="https://user-images.githubusercontent.com/3173176/165888866-d67829fc-7b82-4d95-b36e-47a2c3fcea24.png">

## Development

We'd love any contributions!

To get started see [docs/development.md](docs/development.md)

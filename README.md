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
* Runnable standalone or via http://doctree.org (not yet online)

## Try it out (**EXTREMELY** early stages)

Probably not worth trying out yet unless you're *incredibly* excited about this idea.

### Installation

<details>
<summary>macOS (Apple Silicon)</summary>

```sh
curl -L https://github.com/sourcegraph/doctree/releases/latest/download/doctree-aarch64-macos -o /usr/local/bin/doctree
chmod +x /usr/local/bin/doctree
```

</details>

<details>
<summary>macOS (Intel)</summary>

```sh
curl -L https://github.com/sourcegraph/doctree/releases/latest/download/doctree-x86_64-macos -o /usr/local/bin/doctree
chmod +x /usr/local/bin/doctree
```

</details>

<details>
<summary>Linux (x86_64)</summary>

```sh
curl -L https://github.com/sourcegraph/doctree/releases/latest/download/doctree-x86_64-linux -o /usr/local/bin/doctree
chmod +x /usr/local/bin/doctree
```

</details>

<details>
<summary>Windows (x86_64)</summary>
In an administrator PowerShell, run:

```powershell
New-Item -ItemType Directory 'C:\Program Files\Sourcegraph'

Invoke-WebRequest https://github.com/sourcegraph/doctree/releases/latest/download/doctree-x86_64-windows.exe -OutFile 'C:\Program Files\Sourcegraph\doctree.exe'

[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', [EnvironmentVariableTarget]::Machine) + ';C:\Program Files\Sourcegraph', [EnvironmentVariableTarget]::Machine)
$env:Path += ';C:\Program Files\Sourcegraph'
```

Or download [the exe file](https://github.com/sourcegraph/doctree/releases/latest/download/doctree-x86_64-windows.exe) and install it wherever you like.

</details>

<details>
<summary>Via Docker</summary>

```sh
docker run -it --publish 3333:3333 --rm --name doctree --volume ~/.doctree:/home/nonroot/.doctree sourcegraph/doctree:latest
```

In a folder with Go code you'd like to see docs for, index it (for a large project like `golang/go` expect it to take ~52s for now. It's not multi-threaded.):

```sh
docker run -it --volume $(pwd):/index --volume ~/.doctree:/home/nonroot/.doctree --entrypoint=sh sourcegraph/doctree:latest -c "cd /index && doctree index ."
```

</details>

<details>
<summary>DigitalOcean user data</summary>

```sh
#!/bin/bash

apt update -y && apt upgrade -y && apt install -y docker.io
apt install -y git

mkdir -p $HOME/.doctree && chown 10000:10001 -R $HOME/.doctree

# Index golang/go repository
git clone https://github.com/golang/go
chown 10000:10001 -R go
cd go
docker run -i --volume $(pwd):/index --volume $HOME/.doctree:/home/nonroot/.doctree --entrypoint=sh sourcegraph/doctree:latest -c "cd /index && doctree index ."

# Run server
docker rm -f doctree || true
docker run -d --rm --name doctree -p 80:3333 --volume $HOME/.doctree:/home/nonroot/.doctree sourcegraph/doctree:latest
```

</details>

### Usage

Run the server:

```sh
doctree serve
```

Index a Go project (takes ~52s for a large project like golang/go itself, will be improved soon):

```sh
doctree index .
```

Navigate to http://localhost:3333

## Screenshots

<img width="976" alt="image" src="https://user-images.githubusercontent.com/3173176/165888825-b9399cb1-7025-4242-9bcd-5773b6382fff.png">

<img width="976" alt="image" src="https://user-images.githubusercontent.com/3173176/165888831-cdc5cd87-7d9c-4465-bf9a-71e019f3f9bb.png">

<img width="1267" alt="image" src="https://user-images.githubusercontent.com/3173176/165888866-d67829fc-7b82-4d95-b36e-47a2c3fcea24.png">

## Development

We'd love any contributions!

To get started see [docs/development.md](docs/development.md)

# doctree: 100% open-source library docs tool for every language

doctree provides first-class library documentation for every language (based on tree-sitter), with symbol search & more. If connected to Sourcegraph, it can automatically surface real-world usage examples.

## Try it at [doctree.org](https://doctree.org)

[![](https://user-images.githubusercontent.com/3173176/168915777-571410e3-ef6e-486d-86a7-dea926246d2c.png)](https://doctree.org)

## Run locally, self-host, or use doctree.org

doctree is a single binary, lightweight, and designed to run on your local machine. It can be self-hosted, and used via doctree.org with any GitHub repository.

## Experimental! Early stages!

Extremely early stages, we're working on adding more languages, polishing the experience, and adding usage examples. It's all very early and not yet ready for production use, please bear with us!

Please see [the v1.0 roadmap](https://github.com/sourcegraph/doctree/issues/27) for more, ideas welcome!

## Join us on Discord

If you think what we're building is a good idea, we'd love to hear your thoughts!
[Discord invite](https://discord.gg/vqsBW8m5Y8)

## Language support

Adding support for more languages is easy. To request support for a language [comment on this issue](https://github.com/sourcegraph/doctree/issues/10)

| language | functions | types | methods | consts/vars | search | usage examples | code intel |
|----------|-----------|-------|---------|-------------|--------|----------------|------------|
| Go       | ✅        | ❌     | ❌       | ❌          | ✅     | ❌             | ❌          |
| Python   | ✅        | ❌     | ❌       | ❌          | ✅     | ❌             | ❌          |
| Zig      | ✅        | ❌     | partial | ❌          | ✅     | ❌              | ❌          |
| Markdown | n/a       | ❌     | n/a     | n/a         | ✅     | n/a            | n/a        |

## Installation

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

## Usage

Run the server:

```sh
doctree serve
```

Index a Go project (takes ~52s for a large project like golang/go itself, will be improved soon):

```sh
doctree index .
```

Navigate to http://localhost:3333

## Contributing

We'd love any contributions!

To get started see [docs/development.md](docs/development.md) and the [language support tracking issue](https://github.com/sourcegraph/doctree/issues/10).

## Changelog

### v0.1 (not yet released)

#### Added

* Go, Python, Zig, and Markdown basic support
* Basic search navigation experience based on [experimental Sinter search filters](https://github.com/hexops/sinter/blob/c87e502f3cfd468d3d1263b7caf7cea94ff6d084/src/filter.zig#L18-L85)
* Searching globally across all projects, and within specific projects is now possible.
* Searching within a specific language is now supported (add "go", "python", "md" / "markdown" to front of your query string.)
* Markdown files now have headers and sub-headers indexed for search (e.g. `# About doctree > Installation` shows up in search)
* Basic Markdown frontmatter support.
* Initial [doctree schema format](https://github.com/sourcegraph/doctree/blob/main/doctree/schema/schema.go)
* Experimental (not yet ready for use) auto-indexing, `doctree add .` to monitor a project for file changes and re-index automatically.
* Docker images, single-binary downloads for every OS cross-compiled via Zig compiler.
* Initial [v1.0 roadmap](https://github.com/sourcegraph/doctree/issues/27), [language support tracking issue](https://github.com/sourcegraph/doctree/issues/10)

Special thanks: [@KShivendu](https://github.com/KShivendu) (Python support), [@slimsag](https://github.com/slimsag) (Zig support)

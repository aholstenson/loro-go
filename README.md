# loro-go

This repository contains [Loro CRDT](https://github.com/loro-dev/loro) bindings for Go. Contains pre-built binaries for MacOS (ARM64, AMD64) and Linux (ARM64, AMD64).

⚠️ There is currently very little extra plumbing to make the bindings easier to use. I'm updating it as I need it.

Current Loro version: 1.10.3

## Updating the Loro version

```console
./scripts/updateLoro.sh 1.x.x
```

You can get the version number from the [loro-ffi tags](https://github.com/loro-dev/loro-ffi/tags) page.

This updates the `loro-ffi` submodule, `loro-go/Cargo.toml`, and the lockfile. Open a PR with the changes and CI will build the libraries and commit them after merge.

## Usage

```console
go get github.com/aholstenson/loro-go
```

When building your binary you will likely want statically linking to avoid a dependency on libgcc:

```console
go build -ldflags '-linkmode external -extldflags "-static"'
```

## Examples

### Getting started

```go
doc := loro.NewLoroDoc()

loroMap := doc.GetMap(loro.AsContainerID("test"))
loroMap.Insert("test", loro.AsStringValue("test"))
```

### Exporting updates

```go
updates, err := doc.ExportUpdates(loro.NewVersionVector())
```

### Importing updates

```go
status, err := doc.Import(updates)
```

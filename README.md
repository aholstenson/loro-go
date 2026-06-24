# loro-go

[![Release](https://img.shields.io/github/v/release/aholstenson/loro-go)](https://github.com/aholstenson/loro-go/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/aholstenson/loro-go.svg)](https://pkg.go.dev/github.com/aholstenson/loro-go)
[![Go Version](https://img.shields.io/github/go-mod/go-version/aholstenson/loro-go)](go.mod)
[![License](https://img.shields.io/github/license/aholstenson/loro-go)](LICENSE)

This repository contains [Loro CRDT](https://github.com/loro-dev/loro) bindings for Go. Contains pre-built binaries for MacOS (ARM64, AMD64), Linux (ARM64, AMD64), and Windows (AMD64).

Current Loro version: 1.13.6

**Note:** The API is pre-1.0 and may change.

## Usage

```console
go get github.com/aholstenson/loro-go
```

You need CGO enabled to build, but you do not need a Rust toolchain - the pre-built static libraries are included in this module.

On Linux you will likely want to statically link your binary to avoid a runtime dependency on libgcc:

```console
go build -ldflags '-linkmode external -extldflags "-static"'
```

## Examples

### Getting started

```go
doc := loro.NewLoroDoc()

m := doc.GetMap(loro.AsContainerId("settings"))
m.InsertAny("theme", "dark")

theme, ok := m.GetString("theme")
```

### Nested containers

```go
m := doc.GetMap(loro.AsContainerId("doc"))

users, err := m.GetOrCreateLoroMap("users")
alice, err := users.GetOrCreateLoroMap("alice")
alice.InsertAny("name", "Alice")
```

### Collaborative text

```go
note := doc.GetText(loro.AsContainerId("note"))
note.Insert(0, "Hello, world!")
note.Insert(7, "Loro ")
// note.String() == "Hello, Loro world!"
```

### Syncing two documents

```go
a := loro.NewLoroDoc()
b := loro.NewLoroDoc()

// Send everything b is missing from a.
updates, err := a.Export(loro.UpdatesMode(b.StateVv()))
status, err := b.Import(updates)

// For a fresh peer, send a full snapshot instead:
snapshot, err := a.Export(loro.SnapshotMode())
```

### Merging concurrent edits

Two docs can edit independently and converge after exchanging updates:

```go
a := loro.NewLoroDoc()
b := loro.NewLoroDoc()

a.GetMap(loro.AsContainerId("m")).InsertAny("from-a", int64(1))
b.GetMap(loro.AsContainerId("m")).InsertAny("from-b", int64(2))

aUpdates, _ := a.Export(loro.UpdatesMode(b.StateVv()))
bUpdates, _ := b.Export(loro.UpdatesMode(a.StateVv()))

b.Import(aUpdates)
a.Import(bUpdates)

// Both docs now contain from-a and from-b.
```

## Updating the Loro version

Ensure you use `--recursive` when you `git clone` this repository to pull in the [`loro-ffi`](https://github.com/loro-dev/loro-ffi) submodule.

```console
./scripts/updateLoro.sh 1.x.x
```

You can get the version number from the [loro-ffi tags](https://github.com/loro-dev/loro-ffi/tags) page.

This updates the `loro-ffi` submodule, `loro-go/Cargo.toml`, and the lockfile. Open a PR with the changes and CI will build the libraries and commit them after merge.

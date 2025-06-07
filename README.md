# loro-go

This repository contains [Loro CRDT](https://github.com/loro-dev/loro) bindings for Go. Contains pre-built binaries for MacOS (ARM64, AMD64) and Linux (ARM64, AMD64).

⚠️ There is currently very little extra plumbing to make the bindings easier to use. I'm updating it as I need it.

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

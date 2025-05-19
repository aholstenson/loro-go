package loro

// #cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/libs/amd64-apple-darwin -lloro
// #cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/libs/aarch64-apple-darwin -lloro
// #cgo linux,amd64 LDFLAGS: -L${SRCDIR}/libs/amd64-unknown-linux-musl -lloro
// #cgo linux,arm64 LDFLAGS: -L${SRCDIR}/libs/aarch64-unknown-linux-musl -lloro
import "C"

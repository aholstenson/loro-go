#!/usr/bin/env bash
set -euo pipefail

if [ $# -ne 1 ]; then
	echo "Usage: $0 <version>"
	echo "Example: $0 1.10.3"
	exit 1
fi

VERSION="$1"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Update loro-ffi submodule
echo "Updating loro-ffi submodule to v${VERSION}..."
cd "$REPO_ROOT/loro-ffi"
git fetch --tags
git checkout "v${VERSION}"

# Update Cargo.toml
echo "Updating loro-go/Cargo.toml..."
cd "$REPO_ROOT"
sed -i '' "s/^version = \".*\"/version = \"${VERSION}\"/" loro-go/Cargo.toml
sed -i '' "s/^\(loro-ffi = .*tag = \"\)v[^\"]*\(\".*\)$/\1v${VERSION}\2/" loro-go/Cargo.toml
if ! grep -q "loro-ffi = .*tag = \"v${VERSION}\"" loro-go/Cargo.toml; then
	echo "ERROR: failed to update loro-ffi tag to v${VERSION} in loro-go/Cargo.toml" >&2
	exit 1
fi

# Update README
echo "Updating README.md..."
sed -i '' "s/^Current Loro version: .*/Current Loro version: ${VERSION}/" README.md
if ! grep -q "^Current Loro version: ${VERSION}$" README.md; then
	echo "ERROR: failed to update Loro version to ${VERSION} in README.md" >&2
	exit 1
fi

# Verify our uniffi pin matches loro-ffi's. Drift here causes uniffi-bindgen-go
# to silently produce no output (exit 0, no files) — see commit history.
FFI_UNIFFI=$(awk -F'"' '/^uniffi *= *\{ *version *= */ {print $2; exit}' loro-ffi/Cargo.toml)
GO_UNIFFI=$(awk -F'"' '/^uniffi *= *\{ *version *= */ {print $2; exit}' loro-go/Cargo.toml)
if [ -z "$FFI_UNIFFI" ] || [ -z "$GO_UNIFFI" ]; then
	echo "ERROR: could not parse uniffi version from one of the Cargo.toml files" >&2
	exit 1
fi
FFI_MINOR=$(echo "$FFI_UNIFFI" | cut -d. -f1-2)
GO_MINOR=$(echo "$GO_UNIFFI" | cut -d. -f1-2)
if [ "$FFI_MINOR" != "$GO_MINOR" ]; then
	echo "ERROR: uniffi version mismatch — loro-ffi pins $FFI_UNIFFI, loro-go/Cargo.toml pins $GO_UNIFFI." >&2
	echo "Bump loro-go/Cargo.toml's uniffi pin to ${FFI_UNIFFI} and the uniffi-bindgen-go tag to a release matching uniffi ${FFI_MINOR}.x" >&2
	echo "(see https://github.com/NordSecurity/uniffi-bindgen-go/releases for the right tag)." >&2
	exit 1
fi

# The uniffi-bindgen-go tag encodes the uniffi version it targets as "+vX.Y.Z".
# Make sure that matches our uniffi pin's minor.
BINDGEN_UNIFFI=$(awk -F'"' '/uniffi-bindgen-go *=/ {for (i=1;i<=NF;i++) if ($i ~ /\+v/) {sub(/.*\+v/, "", $i); print $i; exit}}' loro-go/Cargo.toml)
if [ -n "$BINDGEN_UNIFFI" ]; then
	BINDGEN_MINOR=$(echo "$BINDGEN_UNIFFI" | cut -d. -f1-2)
	if [ "$BINDGEN_MINOR" != "$GO_MINOR" ]; then
		echo "ERROR: uniffi-bindgen-go tag targets uniffi ${BINDGEN_UNIFFI}, but uniffi pin is ${GO_UNIFFI}." >&2
		echo "Update the uniffi-bindgen-go tag in loro-go/Cargo.toml to a release matching uniffi ${GO_MINOR}.x." >&2
		exit 1
	fi
fi

# Update lockfile
echo "Updating Cargo.lock..."
cargo update --manifest-path loro-go/Cargo.toml

echo "Done! Review the changes and open a PR."

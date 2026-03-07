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
sed -i.bak "s/^version = \".*\"/version = \"${VERSION}\"/" loro-go/Cargo.toml
sed -i.bak "s/loro-ffi = { git = \".*\", tag = \".*\" }/loro-ffi = { git = \"https:\/\/github.com\/loro-dev\/loro-ffi.git\", tag = \"v${VERSION}\" }/" loro-go/Cargo.toml
rm -f loro-go/Cargo.toml.bak

# Update lockfile
echo "Updating Cargo.lock..."
cargo update --manifest-path loro-go/Cargo.toml

echo "Done! Review the changes and open a PR."

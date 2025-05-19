#!/usr/bin/env bash

# This script was cribbed from https://github.com/automerge/automerge-swift/blob/main/scripts/build-xcframework.sh
# which was cribbed from https://github.com/y-crdt/y-uniffi/blob/7cd55266c11c424afa3ae5b3edae6e9f70d9a6bb/lib/build-xcframework.sh
# which was written by Joseph Heck and  Aidar Nugmanoff and licensed under the MIT license.

set -euxo pipefail
THIS_SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
LIB_NAME="libloro.a"

RUST_FOLDER="$THIS_SCRIPT_DIR/../loro-rs"
GO_FOLDER="$THIS_SCRIPT_DIR/../"

TARGETS="x86_64-unknown-linux-musl aarch64-unknown-linux-musl aarch64-apple-darwin x86_64-apple-darwin"

echo "▸ Install toolchains"
for TARGET in $TARGETS; do
    rustup target add $TARGET
done
cargo_build="cargo build --manifest-path $RUST_FOLDER/Cargo.toml"

echo "▸ Generate Go bindings"
cd "$RUST_FOLDER"
cargo run \
    --features=cli \
    --bin uniffi-bindgen-go \
    "$RUST_FOLDER/src/loro.udl" \
    --out-dir "target/go"

cp -r "${RUST_FOLDER}/target/go/loro/" "${GO_FOLDER}"

for TARGET in $TARGETS; do
    echo "▸ Building for $TARGET"
    $cargo_build --target $TARGET --locked --release

	mkdir -p "${GO_FOLDER}/libs/${TARGET}"
	cp "${RUST_FOLDER}/target/${TARGET}/release/${LIB_NAME}" "${GO_FOLDER}/libs/${TARGET}/${LIB_NAME}"
done

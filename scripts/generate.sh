#!/usr/bin/env bash
set -euxo pipefail
THIS_SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
LIB_NAME="libloro.a"

RUST_FOLDER="$THIS_SCRIPT_DIR/../loro-rs"
GO_FOLDER="$THIS_SCRIPT_DIR/../"

TARGETS="x86_64-unknown-linux-musl aarch64-unknown-linux-musl aarch64-apple-darwin x86_64-apple-darwin"

if ! command -v cross &> /dev/null; then
    echo "▸ Cross tool not found, installing"
    cargo install cross --git https://github.com/cross-rs/cross
fi

echo "▸ Generate Go bindings"
cd "$RUST_FOLDER"
cargo run \
    --features=cli \
    --bin uniffi-bindgen-go \
    "$RUST_FOLDER/src/loro.udl" \
    --out-dir "target/go"

cp -r "${RUST_FOLDER}/target/go/loro/" "${GO_FOLDER}"

export RUSTFLAGS="-C target-feature=+crt-static"

for TARGET in $TARGETS; do
    echo "▸ Building for $TARGET"
    
	cross build --manifest-path "$RUST_FOLDER/Cargo.toml" --target "$TARGET" --locked --release

	mkdir -p "${GO_FOLDER}/libs/${TARGET}"
	cp "${RUST_FOLDER}/target/${TARGET}/release/${LIB_NAME}" "${GO_FOLDER}/libs/${TARGET}/${LIB_NAME}"
done

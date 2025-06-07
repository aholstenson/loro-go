#!/usr/bin/env bash
set -euxo pipefail
THIS_SCRIPT_DIR="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
LIB_NAME="libloro.a"

RUST_FOLDER="$THIS_SCRIPT_DIR/../loro-rs"
GO_FOLDER="$THIS_SCRIPT_DIR/../"

TARGETS="x86_64-unknown-linux-musl aarch64-unknown-linux-musl aarch64-apple-darwin x86_64-apple-darwin"

# Function to check if we're on the right platform for native compilation
can_build_natively() {
    local target="$1"
    local current_arch=$(uname -m)
    local current_os=$(uname -s)
    
    case "$target" in
        "x86_64-apple-darwin")
            [[ "$current_os" == "Darwin" ]]
            ;;
        "aarch64-apple-darwin")
            [[ "$current_os" == "Darwin" ]]
            ;;
        "x86_64-unknown-linux-musl")
            [[ "$current_os" == "Linux" && "$current_arch" == "x86_64" ]]
            ;;
        "aarch64-unknown-linux-musl")
            [[ "$current_os" == "Linux" && "$current_arch" == "aarch64" ]]
            ;;
        *)
            false
            ;;
    esac
}

echo "▸ Install toolchains"
for TARGET in $TARGETS; do
	if can_build_natively "$TARGET"; then
		rustup target add $TARGET
	fi
done

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

for TARGET in $TARGETS; do
    echo "▸ Building for $TARGET"
    
    # Choose the right build tool based on target and current platform
    if can_build_natively "$TARGET"; then
        echo "  Using native cargo build for $TARGET"
        cargo build --manifest-path "$RUST_FOLDER/Cargo.toml" --target "$TARGET" --locked --release
    else
        echo "  Using cross for $TARGET"
        cross build --manifest-path "$RUST_FOLDER/Cargo.toml" --target "$TARGET" --locked --release
    fi

	mkdir -p "${GO_FOLDER}/libs/${TARGET}"
	cp "${RUST_FOLDER}/target/${TARGET}/release/${LIB_NAME}" "${GO_FOLDER}/libs/${TARGET}/${LIB_NAME}"
done

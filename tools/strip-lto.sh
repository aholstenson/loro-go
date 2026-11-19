#!/usr/bin/env bash
# Strip LLVM bitcode from a built static library.
#
# Go 1.27.4 hardened the cgo linker: the go command now refuses to pass a
# library to the C linker if a member has an __LLVM (or __GNU_LTO) segment, or
# a section whose name starts with .gnu.lto_ / .gnu.debuglto_ / .llvm.lto, or is
# .llvmbc / .llvmcmd (see cmd/go/internal/work/security.go). The archives this
# repo builds fail that check, so linking them from cgo fails with e.g.
#
#   disallowed LTO section __LLVM,__bitcode in #cgo LDFLAGS: .../libloro.a
#
# The bitcode is not needed: every object in these archives also carries the
# machine code the linker uses. Removing the IR keeps the exported C API
# byte-for-byte identical, which this script verifies before writing the file.
#
# On macOS the transform is `ld -r` + `libtool -static`: the Apple linker drops
# the __LLVM segment while relocating. On ELF it is `objcopy
# --remove-section` per member, then re-archive.
#
# Only the platform limits the transform: Mach-O needs Apple's linker, ELF needs
# binutils. Run it on the same OS that produced the library, which is what
# .github/workflows/build-libs.yml does.
#
#   tools/strip-lto.sh <path/to/libloro.a>
#
# TOOL_PREFIX addresses a cross toolchain, e.g. TOOL_PREFIX=aarch64-linux-gnu-.
set -euo pipefail

lib=${1:?usage: tools/strip-lto.sh <path/to/libloro.a>}
[ -f "$lib" ] || { echo "no such archive: $lib" >&2; exit 1; }
# Absolute, because the transform runs inside a mktemp -d scratch directory.
lib=$(cd "$(dirname "$lib")" && pwd)/$(basename "$lib")

prefix=${TOOL_PREFIX:-}

# The exported C API is the one thing that must not change. Uniffi's entry
# points are all `ffi_loro*`, so counting them is enough to catch a transform
# that silently dropped or duplicated objects.
api() {
	"${prefix}nm" --defined-only -g "$1" 2>/dev/null | grep -Ec ' T _?ffi_loro' || true
}

before=$(api "$lib")
[ "$before" -gt 0 ] || {
	echo "could not read the ffi_loro API out of $lib (wrong TOOL_PREFIX?)" >&2
	exit 1
}

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# The archive format, not the OS, decides the tool: a macOS host can hold an
# ELF archive and vice versa. Sniff the first member instead of uname.
# No `| head -1` here: with pipefail, head closing the pipe early makes `ar`
# die of SIGPIPE and the script exit 141. Read the list, then take the first.
members=$(ar t "$lib" 2>/dev/null | grep -v '^/$\|^__.SYMDEF' || true)
first_member=$(printf '%s\n' "$members" | awk 'NR==1')
[ -n "$first_member" ] || { echo "could not read any member out of $lib" >&2; exit 1; }
ar p "$lib" "$first_member" > "$work/probe" 2>/dev/null

magic=$(od -An -tx1 -N2 "$work/probe" | tr -d " ")
case "$magic" in
cffa|cefa|feedface*|feedfacf*) format=macho ;;
4c01|6486)                     format=coff ;;
*)                             format=elf ;;
esac

case "$format" in
macho)
	(cd "$work" && ar x "$lib" && rm -f probe)
	# One relocatable object out of all members; the Apple linker drops the
	# __LLVM segment during the merge. The arch comes from the archive, not
	# from this host: a checkout can hold both slices and a cross build must
	# not be merged into the host's architecture.
	arch=$(lipo -archs "$lib" 2>/dev/null | awk '{print $1}')
	[ -n "$arch" ] || arch=arm64
	xcrun ld -r -arch "$arch" -o "$work/merged.o" "$work"/*.o
	xcrun libtool -static -o "$lib.tmp" "$work/merged.o"
	;;
elf)
	mkdir "$work/out"
	(cd "$work" && ar x "$lib" && rm -f probe)
	for obj in "$work"/*.o; do
		"${prefix}objcopy" \
			--remove-section=.llvmbc --remove-section=.llvmcmd \
			--remove-section=.llvm.lto "$obj" "$work/out/$(basename "$obj")"
	done
	rm -f "$lib.tmp"
	(cd "$work/out" && "${prefix}ar" rcs "$lib.tmp" ./*.o)
	;;
coff)
	# llvm-objcopy understands COFF and strips the same section names; binutils
	# objcopy for a mingw target does too when built with COFF support.
	mkdir "$work/out"
	(cd "$work" && ar x "$lib" && rm -f probe)
	for obj in "$work"/*.o; do
		"${prefix}objcopy" \
			--remove-section=.llvmbc --remove-section=.llvmcmd \
			--remove-section=.llvm.lto "$obj" "$work/out/$(basename "$obj")"
	done
	rm -f "$lib.tmp"
	(cd "$work/out" && "${prefix}ar" rcs "$lib.tmp" ./*.o)
	;;
esac

after=$(api "$lib.tmp")
if [ "$before" != "$after" ]; then
	echo "exported API changed: $before -> $after ffi_loro symbols; leaving $lib untouched" >&2
	exit 1
fi

# Gate on the artefact Go will actually see, not on the transform's opinion.
if [ "$format" = macho ]; then
	# otool exits non-zero on archives even when it prints every member, and
	# `set -o pipefail` turns a `otool | grep -q` into a false pass. Capture
	# first, match on the text.
	# (`grep -q` closes the pipe early, which pipefail also reports as failure,
	# so match without -q and on the already-captured text.)
	loads=$(otool -l "$lib.tmp" 2>/dev/null || true)
	if [ -n "$(printf '%s' "$loads" | grep '__LLVM')" ]; then
		echo "a Mach-O __LLVM segment survived; leaving $lib untouched" >&2
		exit 1
	fi
else
	# ELF and COFF: read the section table without assuming a toolchain. The
	# names the go command rejects are fixed (cmd/go/internal/work/security.go
	# isDisallowedLTOSection), so scan the raw bytes of every member for them.
	mkdir "$work/verify"
	(cd "$work/verify" && ar x "$lib.tmp" 2>/dev/null || true)
	if grep -rlE '\.gnu\.lto_|\.gnu\.debuglto_|\.llvm\.lto|\.llvmbc|\.llvmcmd' \
		"$work/verify" >/dev/null 2>&1; then
		echo "an LTO section survived in $format members; leaving $lib untouched" >&2
		exit 1
	fi
fi

mv "$lib.tmp" "$lib"
echo "ok: $lib carries no LTO IR, $after exported ffi_loro symbols intact"

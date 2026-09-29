#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
archive="$root/third_party/mdbtools-1.0.1.tar.gz"
expected=ff9c425a88bc20bf9318a332eec50b17e77896eef65a0e69415ccb4e396d1812

if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$archive" | cut -d ' ' -f 1)
else
    actual=$(shasum -a 256 "$archive" | cut -d ' ' -f 1)
fi
if [ "$actual" != "$expected" ]; then
    echo "MDB Tools source archive checksum mismatch" >&2
    exit 1
fi

mkdir -p "$root/native"
if [ ! -f "$root/native/mdbtools/configure" ]; then
    tar -xzf "$archive" -C "$root/native"
    mv "$root/native/mdbtools-1.0.1" "$root/native/mdbtools"
fi
cd "$root/native/mdbtools"
if [ ! -f Makefile ]; then
    ./configure --disable-glib --disable-shared --enable-static
fi
make -C src/libmdb
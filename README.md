# go-mdbtools

Read-only Go bindings for [MDB Tools](https://github.com/mdbtools/mdbtools) `libmdb`.
Opens Microsoft Access `.mdb` and `.accdb` files without Access, an ODBC driver,
or the MDB Tools CLI. The library returns native Go values and streams rows so
callers can migrate tables into SQLite using their preferred `database/sql`
driver. It does not write Access files, run Access queries, or automatically
resolve linked tables.

## Build

The official MDB Tools 1.0.1 source archive is bundled under `third_party/`
and verified by SHA-256 before it is built. A C compiler, `make`, `tar`, and
the tools used by `configure` are required. Go must be built with cgo enabled.
No system `mdbtools-devel` installation is needed.

```sh
sh scripts/build-native.sh
go test ./...
```

The script builds a static `libmdb.a` in the ignored `native/` directory. Go
module consumers must run it in a writable checkout (or use a prebuilt copy of
the package); `go get` alone cannot execute native build scripts. Linux x86-64
has been tested. macOS and Windows/MSYS2 builds have not been verified yet.
Static linking has LGPL redistribution obligations: review MDB Tools'
`COPYING.LIB` inside the bundled source before distributing an application.

## API

```go
db, err := mdbtools.Open("project.accdb")
if err != nil { /* handle error */ }

tables, err := db.Tables() // local and linked names, descriptions where available
if err != nil { /* handle error */ }
_ = tables

reader, err := db.Scan("Project_Env") // local tables only
if err != nil { /* handle error */ }
defer reader.Close()

for reader.Next() {
    row := reader.Row() // values match reader.Columns in order
    // Insert the row in a SQLite transaction with parameterized placeholders.
    _ = row
}
if err := reader.Err(); err != nil { /* handle error */ }
```

Values are `nil` for NULL, `bool`, `int64`, `float64`, `time.Time`, `string`,
or `[]byte`. Decimal/currency values and GUIDs remain strings to avoid loss of
precision. Access dates have no timezone; `time.Time` uses UTC as a neutral
location, not as a claim that the original timestamp was UTC. `ReadAll` is
available for small tables; use `Scan` for migration. Values exceeding the
text binding limit (65 KiB), memos with more than 32 KiB of source bytes, and
blobs over 32 MiB return errors rather than silently truncating data.
Fractional seconds in Access dates are not yet retained.

For a VPRO import, use `Tables` to inventory local tables and linked names;
stream each local table into a **new** SQLite file in a transaction, preserving
names, NULLs, descriptions, and row counts. Publish the file only after
verifying it against the Access source. This package provides the source reader,
not VPRO's conversion, schema-upgrade, or SQLite policy.

## Tests

`go test ./...` checks the API. Optional integration tests use a local public
Northwind MDB fixture and a VPRO service-pack ACCDB fixture, neither of which
is included in this repository:

```sh
MDBTOOLS_TEST_DB=/path/to/nwind.mdb \
MDBTOOLS_TEST_ACCDB=/path/to/Vp19SpList.accdb go test ./... -v
```

The Go adapter is MIT-licensed. The bundled MDB Tools library has its own
LGPL license; its CLI programs are not part of the Go binding.
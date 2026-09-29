package mdbtools

/*
#cgo CFLAGS: -I${SRCDIR}/native/mdbtools/include
#cgo LDFLAGS: ${SRCDIR}/native/mdbtools/src/libmdb/.libs/libmdb.a -lm
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
	"unsafe"
)

// DB refers to an Access file. Each query opens an independent, read-only handle.
type DB struct{ path string }

// Open verifies that path is a readable Access database.
func Open(path string) (*DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	name := C.CString(path)
	defer C.free(unsafe.Pointer(name))
	handle := C.mdb_open(name, C.MDB_NOFLAGS)
	if handle == nil {
		return nil, fmt.Errorf("open Access database %q: unsupported or unreadable file", path)
	}
	C.mdb_close(handle)
	return &DB{path: path}, nil
}

// Tables lists local and linked table names without following linked tables.
func (db *DB) Tables() ([]Table, error) {
	name := C.CString(db.path)
	defer C.free(unsafe.Pointer(name))
	handle := C.mdb_open(name, C.MDB_NOFLAGS)
	if handle == nil {
		return nil, errors.New("cannot reopen Access database")
	}
	defer C.mdb_close(handle)
	catalog := C.mdb_read_catalog(handle, C.MDB_ANY)
	if catalog == nil {
		return nil, errors.New("cannot read Access catalog")
	}
	tables := make([]Table, 0)
	for index := C.guint(0); index < catalog.len; index++ {
		entry := C.go_mdb_entry(catalog, C.uint(index))
		if entry.object_type != C.MDB_TABLE && entry.object_type != C.MDB_LINKED_TABLE {
			continue
		}
		if entry.object_type == C.MDB_TABLE && C.mdb_is_user_table(entry) == 0 {
			continue
		}
		table := Table{Name: C.GoString(&entry.object_name[0]), Linked: entry.object_type == C.MDB_LINKED_TABLE}
		if !table.Linked {
			definition := C.mdb_read_table(entry)
			if definition != nil {
				key := C.CString("Description")
				table.Description = C.GoString(C.mdb_table_get_prop(definition, key))
				C.free(unsafe.Pointer(key))
				C.mdb_free_tabledef(definition)
			}
		}
		tables = append(tables, table)
	}
	return tables, nil
}

// Reader streams one local table. Close it when finished.
type Reader struct {
	native  *C.GoMdbReader
	Columns []Column
	row     Row
	err     error
}

// Scan opens a local table for sequential iteration. Linked tables cannot be scanned.
func (db *DB) Scan(table string) (*Reader, error) {
	path := C.CString(db.path)
	name := C.CString(table)
	defer C.free(unsafe.Pointer(path))
	defer C.free(unsafe.Pointer(name))
	native := C.go_mdb_open_reader(path, name)
	if native == nil {
		return nil, fmt.Errorf("open local Access table %q: table missing or unreadable", table)
	}
	reader := &Reader{native: native}
	for index := C.int(0); index < C.int(native.table.num_cols); index++ {
		column := C.go_mdb_column(native, index)
		reader.Columns = append(reader.Columns, Column{
			Name: C.GoString(&column.name[0]), Type: columnType(column.col_type),
		})
	}
	return reader, nil
}

func columnType(kind C.int) string {
	switch kind {
	case C.MDB_BOOL:
		return "boolean"
	case C.MDB_BYTE, C.MDB_INT, C.MDB_LONGINT:
		return "integer"
	case C.MDB_FLOAT, C.MDB_DOUBLE:
		return "real"
	case C.MDB_MONEY, C.MDB_NUMERIC:
		return "decimal"
	case C.MDB_DATETIME:
		return "datetime"
	case C.MDB_BINARY, C.MDB_OLE:
		return "binary"
	case C.MDB_TEXT, C.MDB_MEMO:
		return "text"
	case C.MDB_REPID:
		return "guid"
	default:
		return "unsupported"
	}
}

// Next advances to the next row. Check Err after it returns false.
func (reader *Reader) Next() bool {
	if reader.native == nil || reader.err != nil {
		return false
	}
	status := C.go_mdb_next(reader.native)
	if status < 0 {
		reader.err = errors.New("could not decode Access row")
		return false
	}
	if status == 0 {
		return false
	}
	reader.row = make(Row, len(reader.Columns))
	for index, column := range reader.Columns {
		if C.go_mdb_is_null(reader.native, C.int(index)) != 0 && column.Type != "boolean" {
			continue
		}
		nativeColumn := C.go_mdb_column(reader.native, C.int(index))
		if C.go_mdb_memo_too_large(reader.native, C.int(index)) != 0 {
			reader.err = fmt.Errorf("Access memo in column %q exceeds safe bind buffer", column.Name)
			return false
		}
		if column.Type == "unsupported" {
			reader.err = fmt.Errorf("unsupported Access type %d in column %q", nativeColumn.col_type, column.Name)
			return false
		}
		if column.Type == "binary" {
			var size C.size_t
			value := C.go_mdb_blob(reader.native, C.int(index), &size)
			if value == nil || uint64(size) > 1<<31-1 {
				C.free(value)
				reader.err = fmt.Errorf("binary column %q is unreadable or too large", column.Name)
				return false
			}
			reader.row[index] = C.GoBytes(value, C.int(size))
			C.free(value)
			continue
		}
		if C.go_mdb_length(reader.native, C.int(index)) >= C.int(reader.native.mdb.bind_size)-1 {
			reader.err = fmt.Errorf("Access value in column %q exceeds bind buffer", column.Name)
			return false
		}
		value := C.GoString(C.go_mdb_text(reader.native, C.int(index)))
		switch column.Type {
		case "boolean":
			if value != "0" && value != "1" {
				reader.err = fmt.Errorf("column %q: invalid Access boolean %q", column.Name, value)
				return false
			}
			reader.row[index] = value == "1"
		case "integer":
			reader.row[index], reader.err = strconv.ParseInt(value, 10, 64)
		case "real":
			reader.row[index], reader.err = strconv.ParseFloat(value, 64)
		case "datetime":
			reader.row[index], reader.err = time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC)
		default:
			reader.row[index] = value // Decimals remain exact text, not float64.
		}
		if reader.err != nil {
			reader.err = fmt.Errorf("column %q: %w", column.Name, reader.err)
			return false
		}
	}
	return true
}

// Row returns a snapshot of the current row; it remains valid after Next.
func (reader *Reader) Row() Row { return reader.row }

// Err reports a decoding error encountered during iteration.
func (reader *Reader) Err() error { return reader.err }

// Close releases all C memory associated with the reader.
func (reader *Reader) Close() error {
	if reader.native != nil {
		C.go_mdb_close_reader(reader.native)
		reader.native = nil
	}
	return nil
}

// ReadAll reads a small table completely into memory. Prefer Scan for imports.
func (db *DB) ReadAll(table string) ([]Column, []Row, error) {
	reader, err := db.Scan(table)
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()
	rows := make([]Row, 0)
	for reader.Next() {
		rows = append(rows, reader.Row())
	}
	return reader.Columns, rows, reader.Err()
}

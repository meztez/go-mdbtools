package mdbtools

// Column describes a field in an Access table.
type Column struct {
	Name string
	Type string
}

// Table describes a local or linked Access table.
type Table struct {
	Name        string
	Linked      bool
	Description string
}

// Row contains values in the same order as a reader's Columns.
// NULL is nil; binary fields are []byte.
type Row []any

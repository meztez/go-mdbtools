package mdbtools

import (
	"os"
	"testing"
	"time"
)

func TestOpenMissing(t *testing.T) {
	if _, err := Open(t.TempDir() + "/missing.mdb"); err == nil {
		t.Fatal("expected an error for a missing database")
	}
}

func TestNorthwind(t *testing.T) {
	path := os.Getenv("MDBTOOLS_TEST_DB")
	if path == "" {
		t.Skip("set MDBTOOLS_TEST_DB to a Northwind Access fixture")
	}
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := db.Tables()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, table := range tables {
		if table.Name == "Shippers" && !table.Linked {
			found = true
		}
	}
	if !found {
		t.Fatal("Shippers table not found")
	}
	reader, err := db.Scan("Shippers")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.Columns) != 3 || reader.Columns[0].Name != "ShipperID" {
		t.Fatalf("unexpected columns: %+v", reader.Columns)
	}
	count := 0
	for reader.Next() {
		count++
		if count == 1 && (reader.Row()[0] != int64(1) || reader.Row()[1] != "Speedy Express") {
			t.Fatalf("unexpected first row: %#v", reader.Row())
		}
	}
	if err := reader.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("Shippers rows = %d, want 3", count)
	}
	employees, err := db.Scan("Employees")
	if err != nil {
		t.Fatal(err)
	}
	defer employees.Close()
	rows, nullRegions, nullManagers := 0, 0, 0
	for employees.Next() {
		rows++
		row := employees.Row()
		if row[9] == nil {
			nullRegions++
		}
		if row[16] == nil {
			nullManagers++
		}
		if _, ok := row[5].(time.Time); !ok {
			t.Fatalf("BirthDate is %T, want time.Time", row[5])
		}
		if photo, ok := row[14].([]byte); !ok || len(photo) == 0 {
			t.Fatalf("Photo is %T, want nonempty []byte", row[14])
		}
	}
	if err := employees.Err(); err != nil {
		t.Fatal(err)
	}
	if rows != 9 || nullRegions != 4 || nullManagers != 1 {
		t.Fatalf("Employees: rows=%d null Regions=%d null ReportsTo=%d", rows, nullRegions, nullManagers)
	}
	products, err := db.Scan("Products")
	if err != nil {
		t.Fatal(err)
	}
	defer products.Close()
	rows, discontinued := 0, 0
	for products.Next() {
		rows++
		row := products.Row()
		if value, ok := row[9].(bool); !ok {
			t.Fatalf("Discontinued is %T, want bool", row[9])
		} else if value {
			discontinued++
		}
		if _, ok := row[5].(string); !ok && row[5] != nil {
			t.Fatalf("UnitPrice is %T, want exact decimal string", row[5])
		}
	}
	if err := products.Err(); err != nil {
		t.Fatal(err)
	}
	if rows != 77 || discontinued != 8 {
		t.Fatalf("Products: rows=%d discontinued=%d", rows, discontinued)
	}
}

func TestACCDB(t *testing.T) {
	path := os.Getenv("MDBTOOLS_TEST_ACCDB")
	if path == "" {
		t.Skip("set MDBTOOLS_TEST_ACCDB to the VPRO service pack list fixture")
	}
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	columns, rows, err := db.ReadAll("USysServicePackList")
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 5 || len(rows) != 1 || rows[0][0] != int64(1582219633) || rows[0][2] != "Reattach project" {
		t.Fatalf("unexpected ACCDB data: columns=%+v rows=%#v", columns, rows)
	}
	if rows[0][3] != time.Date(2020, 2, 20, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("unexpected Access date: %#v", rows[0][3])
	}
}

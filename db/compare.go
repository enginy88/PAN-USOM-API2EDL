package db

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/enginy88/PAN-USOM-API2EDL/logger"

	_ "modernc.org/sqlite"
)

var ErrTypeAssert = errors.New("type assertion failure")

// CompareWithFile compares the in-memory database with a file-based database
func CompareWithFile(dbPath string) (bool, error) {
	// Check if file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return false, fmt.Errorf("database file does not exist: %s", dbPath)
	}

	// Open the file database
	fileDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return false, fmt.Errorf("failed to open file database: %w", err)
	}
	defer fileDB.Close()

	// Ensure the file database is valid
	if err = fileDB.Ping(); err != nil {
		return false, fmt.Errorf("failed to ping file database: %w", err)
	}

	// Compare the databases
	return CompareSQLiteDatabases(db, fileDB)
}

// CompareSQLiteDatabases compares an in-memory SQLite database with a file-based one
func CompareSQLiteDatabases(memoryDB *sql.DB, fileDB *sql.DB) (bool, error) {
	// Compare schema
	schemaSame, err := compareSchema(memoryDB, fileDB)
	if err != nil {
		return false, fmt.Errorf("error comparing schema: %w", err)
	}
	if !schemaSame {
		return false, nil
	}

	// Get all table names
	tables, err := getTables(memoryDB)
	if err != nil {
		return false, fmt.Errorf("error getting tables: %w", err)
	}

	// Compare data in each table
	for _, table := range tables {
		dataSame, err := compareTableData(memoryDB, fileDB, table)
		if err != nil {
			return false, fmt.Errorf("error comparing data in table %s: %w", table, err)
		}
		if !dataSame {
			return false, nil
		}
	}

	return true, nil
}

// Helper functions for schema comparison
func compareSchema(db1, db2 *sql.DB) (bool, error) {
	query := "SELECT type, name, tbl_name, COALESCE(sql, '') FROM sqlite_master ORDER BY type, name"

	rows1, err := db1.Query(query)
	if err != nil {
		return false, err
	}
	defer rows1.Close()

	rows2, err := db2.Query(query)
	if err != nil {
		return false, err
	}
	defer rows2.Close()

	// Compare row by row
	for {
		hasRow1 := rows1.Next()
		hasRow2 := rows2.Next()

		if hasRow1 != hasRow2 {
			return false, nil
		}

		if !hasRow1 {
			break
		}

		var type1, name1, tableName1, sql1 string
		var type2, name2, tableName2, sql2 string

		if err := rows1.Scan(&type1, &name1, &tableName1, &sql1); err != nil {
			return false, err
		}

		if err := rows2.Scan(&type2, &name2, &tableName2, &sql2); err != nil {
			return false, err
		}

		if type1 != type2 || name1 != name2 || tableName1 != tableName2 ||
			normalizeSQL(sql1) != normalizeSQL(sql2) {
			return false, nil
		}
	}

	return true, nil
}

// getTables returns all user table names
func getTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}

// getTableColumns returns column names and types for a table
func getTableColumns(db *sql.DB, tableName string) ([]string, []string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var columns []string
	var types []string
	for rows.Next() {
		var id int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var primaryKey int

		if err := rows.Scan(&id, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, nil, err
		}

		columns = append(columns, name)
		types = append(types, dataType)
	}

	return columns, types, rows.Err()
}

// compareTableData compares all data in a table between databases
func compareTableData(db1, db2 *sql.DB, tableName string) (bool, error) {
	// Get columns to ensure consistent order
	columns, _, err := getTableColumns(db1, tableName)
	if err != nil {
		return false, err
	}

	columnsList := strings.Join(columns, ", ")
	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY rowid DESC", columnsList, tableName)

	rows1, err := db1.Query(query)
	if err != nil {
		return false, err
	}
	defer rows1.Close()

	rows2, err := db2.Query(query)
	if err != nil {
		return false, err
	}
	defer rows2.Close()

	// Compare each row
	for {
		hasRow1 := rows1.Next()
		hasRow2 := rows2.Next()

		if hasRow1 != hasRow2 {
			return false, nil
		}

		if !hasRow1 {
			break
		}

		// Create containers for row values
		values1 := make([]any, len(columns))
		values2 := make([]any, len(columns))
		scanArgs1 := make([]any, len(columns))
		scanArgs2 := make([]any, len(columns))

		for i := range values1 {
			var val1, val2 sql.RawBytes
			scanArgs1[i] = &val1
			scanArgs2[i] = &val2
			values1[i] = &val1
			values2[i] = &val2
		}

		if err := rows1.Scan(scanArgs1...); err != nil {
			return false, err
		}

		if err := rows2.Scan(scanArgs2...); err != nil {
			return false, err
		}

		// Compare values
		for i := range values1 {
			rb1, ok := values1[i].(*sql.RawBytes)
			if !ok {
				logger.LogErr.Println("DB: Type assertion failure! (" + logger.Typeof(values1[i]) + " -> *sql.RawBytes)")
				return false, ErrTypeAssert
			}
			rb2, ok := values2[i].(*sql.RawBytes)
			if !ok {
				logger.LogErr.Println("DB: Type assertion failure! (" + logger.Typeof(values2[i]) + " -> *sql.RawBytes)")
				return false, ErrTypeAssert
			}
			val1 := *rb1
			val2 := *rb2

			// If both are NULL (nil)
			if val1 == nil && val2 == nil {
				continue
			}

			// If one is NULL but not the other
			if (val1 == nil && val2 != nil) || (val1 != nil && val2 == nil) {
				return false, nil
			}

			// Both are not NULL, compare as bytes
			if !bytes.Equal(val1, val2) {
				return false, nil
			}
		}
	}

	return true, nil
}

// normalizeSQL standardizes SQL for comparison
func normalizeSQL(sql string) string {
	if sql == "" {
		return ""
	}
	return strings.Join(strings.Fields(sql), " ")
}

package db

import (
	"context"
	"database/sql"

	"github.com/enginy88/PAN-USOM-API2EDL/logger"

	_ "modernc.org/sqlite"
)

var db *sql.DB
var dbTime string

func InitDB(ctx context.Context) error {
	var err error

	// Use a file-based database instead of in-memory
	db, err = sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		return err
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS usom_records (
		id INTEGER PRIMARY KEY,
		url TEXT,
		type TEXT,
		desc TEXT,
		source TEXT,
		date TEXT,
		criticality_level INTEGER,
		connection_type TEXT,
		UNIQUE(url, type)
	);
	CREATE INDEX IF NOT EXISTS idx_date ON usom_records(date);
	CREATE INDEX IF NOT EXISTS idx_type ON usom_records(type);
	CREATE INDEX IF NOT EXISTS idx_criticality ON usom_records(criticality_level);
	`

	_, err = db.ExecContext(ctx, createTableSQL)
	return err
}

func GetDBTime() string {
	return dbTime
}

func CloseDB() {
	err := db.Close()
	if err != nil {
		logger.LogWarn.Println("DB: Failed to close database! (" + err.Error() + ")")
	}
}

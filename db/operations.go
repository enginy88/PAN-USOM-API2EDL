package db

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"time"

	"github.com/enginy88/PAN-USOM-API2EDL/logger"
	"github.com/enginy88/PAN-USOM-API2EDL/usom"
)

func BackupToFile(ctx context.Context, dbPath string) error {
	diskDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer diskDB.Close()

	// Use context-aware Ping
	if err := diskDB.PingContext(ctx); err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "vacuum into ?", dbPath)
	if err != nil {
		return err
	}

	return nil
}

func HandleBackupFile(dbPath string) error {
	oldPath := dbPath + "_old"

	// Check if original file exists
	if _, err := os.Stat(dbPath); err == nil {
		// Remove old backup if exists
		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			logger.LogWarn.Println("DB: Failed to remove old backup file! (" + err.Error() + ")")
		}
		// Rename current file to backup
		if err := os.Rename(dbPath, oldPath); err != nil {
			return err
		}
	}
	return nil
}

func StoreRecords(ctx context.Context, models []usom.Model) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			logger.LogWarn.Println("DB: Failed to rollback transaction! (" + err.Error() + ")")
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO usom_records 
		(id, url, type, desc, source, date, criticality_level, connection_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, model := range models {
		_, err = stmt.ExecContext(ctx, model.ID, model.URL, model.Type, model.Desc,
			model.Source, model.Date, model.CriticalityLevel, model.ConnectionType)
		if err != nil {
			return err
		}
	}

	dbTime = time.Now().UTC().Format(time.RFC1123)

	return tx.Commit()
}

func LoadFromFile(ctx context.Context, dbPath string) error {
	fileDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer fileDB.Close()

	// Use context-aware Ping
	if err := fileDB.PingContext(ctx); err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "ATTACH DATABASE ? AS filedb", dbPath)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO usom_records 
		SELECT * FROM filedb.usom_records
	`)
	if err != nil {
		return err
	}

	dbTime = time.Now().UTC().Format(time.RFC1123)

	_, err = db.ExecContext(ctx, "DETACH DATABASE filedb")

	return err
}

package db

import (
	"database/sql"
	"os"

	_ "modernc.org/sqlite"
)

func New(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	db.SetMaxOpenConns(1)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func Initialize(db *sql.DB, schema string) error {
	data, err := os.ReadFile(schema)
	if err != nil {
		return err
	}

	_, err = db.Exec(string(data))
	if err != nil {
		return err
	}

	return nil
}

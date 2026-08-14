package db

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSql string

func New(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	db.SetMaxOpenConns(1)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func Initialize(db *sql.DB) error {
	_, err := db.Exec(schemaSql)
	if err != nil {
		return err
	}

	return nil
}

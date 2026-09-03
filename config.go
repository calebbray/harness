package main

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/calebbray/personal-agent/internal/db"
	"github.com/calebbray/personal-agent/internal/logging"
)

var version = "dev"

func loadEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
	return nil
}

func dataDir() (string, error) {
	if dir := os.Getenv("HARNESS_DATA_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create HARNESS_DATA_DIR: %q: %w", dir, err)
		}
		return dir, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config dir: %w", err)
	}

	dir := path.Join(configDir, "harness")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create data dir %q: %w", dir, err)
	}
	return dir, nil
}

func setupLogger(dirname string) (*Logger, error) {
	logPath := path.Join(dirname, "agent.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	fileHandler := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})

	stdoutLevel := slog.LevelWarn
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		stdoutLevel.UnmarshalText([]byte(level))
	}

	stdoutHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: stdoutLevel})

	logger := slog.New(logging.NewFanout(fileHandler, stdoutHandler))
	return &Logger{
		Closer: logFile,
		Logger: logger,
	}, nil
}

func setupDB(dirname string) (*sql.DB, error) {
	dbPath := path.Join(dirname, "agent.db")
	database, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %s", err)
	}
	return database, nil
}

type Logger struct {
	io.Closer
	*slog.Logger
}

func initialize() (*Logger, *sql.DB, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve data directory: %w", err)
	}

	if err := loadEnv(path.Join(dir, ".env")); err != nil {
		return nil, nil, fmt.Errorf("failed to load env: %w", err)
	}

	logger, err := setupLogger(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("logger setup: %w", err)
	}

	database, err := setupDB(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("db setup: %w", err)
	}

	if err := db.Initialize(database); err != nil {
		return nil, nil, fmt.Errorf("database initialization: %w", err)
	}

	return logger, database, nil
}

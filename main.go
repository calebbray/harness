package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/db"
	"github.com/calebbray/personal-agent/internal/logging"
	"github.com/calebbray/personal-agent/internal/repl"
	"github.com/calebbray/personal-agent/internal/tools"
	"github.com/calebbray/personal-agent/internal/workerpool"
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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("harness", version)
		return
	}
	dir, err := dataDir()
	if err != nil {
		log.Fatalf("failed to resolve data directory: %s", err)
	}

	if err := loadEnv(path.Join(dir, ".env")); err != nil {
		log.Fatalf("failed to load env: %s", err)
	}

	logPath := path.Join(dir, "agent.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("failed to open log file: %s", err)
	}
	defer logFile.Close()

	fileHandler := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})

	stdoutLevel := slog.LevelWarn // default
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		stdoutLevel.UnmarshalText([]byte(level))
	}

	stdoutHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: stdoutLevel})

	logger := slog.New(logging.NewFanout(fileHandler, stdoutHandler))
	logger.Info("initialized agent log", "path", logPath)

	c, err := client.New("openai", "")
	if err != nil {
		logger.Error("failed to create client", "err", err)
	}

	dbPath := path.Join(dir, "agent.db")
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open db: %s", err)
	}
	defer database.Close()
	logger.Info("initialized db", "path", dbPath)

	if err := db.Initialize(database); err != nil {
		logger.Error("could not initialize database", "err", err)
	}

	store := workerpool.NewStore(database)
	store.MarkInterrupted()

	toolRegistry := tools.Default(store)

	a := agent.New(agent.AgentConfig{
		EnforceConfirmation: false,
		Tools:               toolRegistry,
		Client:              c,
		Logger:              logger,
		Database:            database,
	})

	wp := workerpool.New(3, workerpool.TaskConfig{
		Client: c,
		Tools:  toolRegistry,
		Logger: logger,
		Store:  store,
	})
	defer wp.Close()

	wp.Run()

	r := repl.New(repl.ReplConfig{
		Agent:      a,
		Logger:     logger,
		Store:      store,
		WorkerPool: wp,
	})
	r.Run()
}

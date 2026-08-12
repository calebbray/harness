package main

import (
	"log"
	"log/slog"
	"os"
	"path"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/db"
	"github.com/calebbray/personal-agent/internal/logging"
	"github.com/calebbray/personal-agent/internal/repl"
	"github.com/calebbray/personal-agent/internal/tools"
	"github.com/calebbray/personal-agent/internal/workerpool"
)

func main() {
	logFile, err := os.OpenFile("agent.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
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

	c, err := client.New("anthropic")
	if err != nil {
		logger.Error("failed to create client", "err", err)
	}

	database, err := db.New("agent.db")
	if err != nil {
		log.Fatalf("failed to open db: %s", err)
	}
	defer database.Close()

	if err := db.Initialize(database, path.Join("sql", "schema.sql")); err != nil {
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

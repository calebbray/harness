package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/logging"
	"github.com/calebbray/personal-agent/internal/repl"
	"github.com/calebbray/personal-agent/internal/tools"
)

func main() {
	logFile, err := os.OpenFile("agent.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("failed to open log file: %s", err)
	}
	defer logFile.Close()

	fileHandler := slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})

	stdoutLevel := slog.LevelInfo // default
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		stdoutLevel.UnmarshalText([]byte(level))
	}

	stdoutHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: stdoutLevel})

	logger := slog.New(logging.NewFanout(fileHandler, stdoutHandler))

	c, err := client.New()
	if err != nil {
		logger.Error("failed to create client", "err", err)
	}

	toolRegistry := tools.Default()

	a := agent.New(agent.AgentConfig{
		EnforceConfirmation: true,
		Tools:               toolRegistry,
		Client:              c,
		Logger:              logger,
	})

	r := repl.New(repl.ReplConfig{
		Agent:  a,
		Logger: logger,
	})
	r.Run()
}

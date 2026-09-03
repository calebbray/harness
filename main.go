package main

import (
	"fmt"
	"log"
	"os"

	"github.com/calebbray/personal-agent/internal/agent"
	"github.com/calebbray/personal-agent/internal/cli"
	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/repl"
	"github.com/calebbray/personal-agent/internal/tools"
	"github.com/calebbray/personal-agent/internal/workerpool"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("harness", version)
		return
	}

	logger, database, err := initialize()
	if err != nil {
		log.Fatal("could not initialize database and log artifacts", err)
	}
	defer logger.Close()
	defer database.Close()

	c, err := client.Default()
	if err != nil {
		logger.Error("failed to create client", "err", err)
	}

	store := workerpool.NewStore(database)
	store.MarkInterrupted()

	toolRegistry := tools.Default(store)

	a := agent.New(agent.AgentConfig{
		EnforceConfirmation: false,
		Tools:               toolRegistry,
		Client:              c,
		Logger:              logger.Logger,
		Database:            database,
	})

	if len(os.Args) > 1 {
		// cli mode
		c := cli.New(cli.Config{Agent: a, Store: store})
		c.Agent = a
		cmd := os.Args[1]
		args, err := cli.ParseArgs(os.Args[2:])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := c.Run(cmd, args); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		os.Exit(0)
	}

	wp := workerpool.New(3, workerpool.TaskConfig{
		Client: c,
		Tools:  toolRegistry,
		Logger: logger.Logger,
		Store:  store,
	})
	defer wp.Close()

	wp.Run()

	r := repl.New(repl.ReplConfig{
		Agent:      a,
		Logger:     logger.Logger,
		Store:      store,
		WorkerPool: wp,
	})
	r.Run()
}

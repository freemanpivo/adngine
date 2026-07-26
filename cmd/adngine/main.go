package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/freemanpivo/adngine/internal/app"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	flag.Parse()

	a, err := app.New(*configPath)
	if err != nil {
		slog.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}

	if err := a.Run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

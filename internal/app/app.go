package app

import (
	"fmt"
	"log/slog"

	"github.com/freemanpivo/adngine/internal/config"
	"github.com/freemanpivo/adngine/internal/conversation"
	"github.com/freemanpivo/adngine/internal/httpserver"
	"github.com/freemanpivo/adngine/internal/logger"
	"github.com/freemanpivo/adngine/internal/selection"
)

type App struct {
	logger *slog.Logger
	server *httpserver.Server
}

func New(configPath string) (*App, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	log := logger.New(cfg.Log.Level)

	components := cfg.Selection.ComponentNames()
	repo := conversation.NewRepository(log)
	for _, component := range components {
		path := cfg.Selection.Components[component].FilePath
		if err := repo.LoadComponent(component, path); err != nil {
			return nil, fmt.Errorf("loading conversations: %w", err)
		}
		log.Info("component inventory loaded",
			"component", component,
			"file", path,
			"conversations", len(repo.Candidates(component)),
			"timeout", cfg.Selection.Components[component].Timeout.String())
	}

	registry := selection.NewRegistry(repo, components)
	server := httpserver.New(cfg, log, registry)

	return &App{logger: log, server: server}, nil
}

func (a *App) Run() error {
	return a.server.Start()
}

package httpserver

import (
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/freemanpivo/adngine/internal/config"
	"github.com/freemanpivo/adngine/internal/selection"
)

type Server struct {
	app    *fiber.App
	cfg    *config.Config
	logger *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger, registry *selection.Registry) *Server {
	app := fiber.New()

	h := NewHandler(logger, registry)
	app.Post("/v1/selections", h.Select)

	return &Server{app: app, cfg: cfg, logger: logger}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.Server.Port)
	s.logger.Info("starting server", "addr", addr)
	return s.app.Listen(addr)
}

package httpserver

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"github.com/freemanpivo/adngine/internal/conversation"
	"github.com/freemanpivo/adngine/internal/selection"
)

type Handler struct {
	logger   *slog.Logger
	registry *selection.Registry
}

func NewHandler(logger *slog.Logger, registry *selection.Registry) *Handler {
	return &Handler{logger: logger, registry: registry}
}

func (h *Handler) Select(c *fiber.Ctx) error {
	var req selectionRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if len(req.Slots) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "slots is required"})
	}

	client := conversation.Client{
		ID:      req.Client.ID,
		Product: req.Client.Product,
	}

	selections := make(map[string]*conversationDTO, len(req.Slots))
	for _, slot := range req.Slots {
		best, ok := h.registry.Select(slot, client)
		if !ok {
			selections[slot] = nil
			continue
		}
		selections[slot] = toConversationDTO(best)
	}

	return c.JSON(selectionResponseDTO{Selections: selections})
}

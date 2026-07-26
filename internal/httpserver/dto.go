package httpserver

import "github.com/freemanpivo/adngine/internal/conversation"

type selectionRequestDTO struct {
	Client struct {
		ID      string `json:"id"`
		Product string `json:"product,omitempty"`
	} `json:"client"`
	Slots []string `json:"slots"`
}

type conversationDTO struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Product  string `json:"product,omitempty"`
	Text     string `json:"text"`
	Link     string `json:"link"`
	Priority int    `json:"priority"`
}

type selectionResponseDTO struct {
	Selections map[string]*conversationDTO `json:"selections"`
}

func toConversationDTO(c *conversation.Conversation) *conversationDTO {
	if c == nil {
		return nil
	}
	return &conversationDTO{
		ID:       c.ID,
		Type:     string(c.Type),
		Product:  c.Product,
		Text:     c.Text,
		Link:     c.Link,
		Priority: c.Priority,
	}
}

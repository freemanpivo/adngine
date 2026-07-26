package selection

import "github.com/freemanpivo/adngine/internal/conversation"

const ComponentCard = "card"

type CardSelector struct{}

func NewCardSelector() *CardSelector {
	return &CardSelector{}
}

func (s *CardSelector) Select(client conversation.Client, candidates []conversation.Conversation) (*conversation.Conversation, bool) {
	return bestMatch(client, candidates)
}

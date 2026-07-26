package selection

import "github.com/freemanpivo/adngine/internal/conversation"

// DefaultSelector atende os componentes registrados apenas por configuracao,
// sem regra de ranking propria.
type DefaultSelector struct{}

func NewDefaultSelector() *DefaultSelector {
	return &DefaultSelector{}
}

func (s *DefaultSelector) Select(client conversation.Client, candidates []conversation.Conversation) (*conversation.Conversation, bool) {
	return bestMatch(client, candidates)
}

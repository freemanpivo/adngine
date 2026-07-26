package selection

import "github.com/freemanpivo/adngine/internal/conversation"

const ComponentFooter = "footer"

type FooterSelector struct{}

func NewFooterSelector() *FooterSelector {
	return &FooterSelector{}
}

func (s *FooterSelector) Select(client conversation.Client, candidates []conversation.Conversation) (*conversation.Conversation, bool) {
	return bestMatch(client, candidates)
}

package selection

import "github.com/freemanpivo/adngine/internal/conversation"

const ComponentBanner = "banner"

type BannerSelector struct{}

func NewBannerSelector() *BannerSelector {
	return &BannerSelector{}
}

func (s *BannerSelector) Select(client conversation.Client, candidates []conversation.Conversation) (*conversation.Conversation, bool) {
	return bestMatch(client, candidates)
}

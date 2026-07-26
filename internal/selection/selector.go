package selection

import "github.com/freemanpivo/adngine/internal/conversation"

type Selector interface {
	Select(client conversation.Client, candidates []conversation.Conversation) (*conversation.Conversation, bool)
}

type Registry struct {
	repo      *conversation.Repository
	selectors map[string]Selector
}

// Componente sem selector proprio cai no DefaultSelector, para que adicionar um
// componente exija apenas config e arquivo de inventario.
var selectorsByComponent = map[string]func() Selector{
	ComponentBanner: func() Selector { return NewBannerSelector() },
	ComponentCard:   func() Selector { return NewCardSelector() },
	ComponentFooter: func() Selector { return NewFooterSelector() },
}

func NewRegistry(repo *conversation.Repository, components []string) *Registry {
	selectors := make(map[string]Selector, len(components))
	for _, component := range components {
		newSelector, ok := selectorsByComponent[component]
		if !ok {
			newSelector = func() Selector { return NewDefaultSelector() }
		}
		selectors[component] = newSelector()
	}

	return &Registry{repo: repo, selectors: selectors}
}

func (r *Registry) Select(component string, client conversation.Client) (*conversation.Conversation, bool) {
	selector, ok := r.selectors[component]
	if !ok {
		return nil, false
	}
	candidates := r.repo.Candidates(component)
	return selector.Select(client, candidates)
}

// A conversation tied to a product only qualifies when the client is in that
// same product context; among the remaining candidates, highest priority wins.
func bestMatch(client conversation.Client, candidates []conversation.Conversation) (*conversation.Conversation, bool) {
	var best *conversation.Conversation
	for i := range candidates {
		cand := candidates[i]
		if cand.Product != "" && cand.Product != client.Product {
			continue
		}
		if best == nil || cand.Priority > best.Priority {
			best = &candidates[i]
		}
	}
	if best == nil {
		return nil, false
	}
	return best, true
}

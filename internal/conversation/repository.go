package conversation

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type componentInventory struct {
	conversations []Conversation
	fallbacks     map[string]Conversation
}

type Repository struct {
	mu          sync.RWMutex
	logger      *slog.Logger
	inventories map[string]componentInventory
}

func NewRepository(logger *slog.Logger) *Repository {
	return &Repository{
		logger:      logger,
		inventories: make(map[string]componentInventory),
	}
}

func (r *Repository) LoadComponent(component, path string) error {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("lendo inventario de %s (%s): %w", component, path, err)
	}

	var md mapstructure.Metadata
	var inv ComponentInventory
	withMetadata := func(dc *mapstructure.DecoderConfig) { dc.Metadata = &md }
	if err := v.Unmarshal(&inv, withMetadata); err != nil {
		return fmt.Errorf("parseando inventario de %s (%s): %w", component, path, err)
	}

	if err := inv.Validate(component); err != nil {
		return fmt.Errorf("inventario de %s (%s) invalido: %w", component, path, err)
	}

	for _, key := range md.Unused {
		r.logger.Warn("campo desconhecido no inventario, ignorado",
			"component", component, "file", path, "field", key)
	}

	fallbacks := make(map[string]Conversation, len(inv.Fallbacks))
	for _, f := range inv.Fallbacks {
		fallbacks[f.Product] = f
	}

	r.mu.Lock()
	r.inventories[component] = componentInventory{
		conversations: inv.Conversations,
		fallbacks:     fallbacks,
	}
	r.mu.Unlock()

	return nil
}

// Candidates devolve copias na ordem do arquivo: quem consome nao alcanca o
// inventario em memoria.
func (r *Repository) Candidates(component string) []Conversation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inv, ok := r.inventories[component]
	if !ok || len(inv.conversations) == 0 {
		return nil
	}
	return slices.Clone(inv.conversations)
}

// Fallback resolve pelo produto do cliente e, na ausencia de um especifico, pelo
// default.
func (r *Repository) Fallback(component, product string) (*Conversation, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inv, ok := r.inventories[component]
	if !ok {
		return nil, false
	}

	found, ok := inv.fallbacks[product]
	if !ok {
		found, ok = inv.fallbacks[DefaultFallbackProduct]
		if !ok {
			return nil, false
		}
	}
	return &found, true
}

func (r *Repository) Components() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return slices.Sorted(maps.Keys(r.inventories))
}

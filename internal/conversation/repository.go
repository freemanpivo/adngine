package conversation

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

type Repository struct {
	mu            sync.RWMutex
	conversations []Conversation
}

func NewRepository() *Repository {
	return &Repository{}
}

type conversationsFile struct {
	Conversations []Conversation `mapstructure:"conversations"`
}

func (r *Repository) Load(path string) error {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("reading conversations file: %w", err)
	}

	var file conversationsFile
	if err := v.Unmarshal(&file); err != nil {
		return fmt.Errorf("parsing conversations file: %w", err)
	}

	r.mu.Lock()
	r.conversations = file.Conversations
	r.mu.Unlock()
	return nil
}

func (r *Repository) ByComponent(component string) []Conversation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []Conversation
	for _, c := range r.conversations {
		for _, comp := range c.Components {
			if comp == component {
				matches = append(matches, c)
				break
			}
		}
	}
	return matches
}

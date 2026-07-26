package conversation

import (
	"errors"
	"fmt"
	"slices"
)

const DefaultFallbackProduct = ""

var (
	knownTypes   = []Type{TypeKnowledge, TypeAction, TypeEvaluation}
	knownSources = []Source{SourceStatic, SourceDynamoDB, SourceHTTP}
)

// Validate acumula todas as violacoes em vez de parar na primeira, para que uma
// carga quebrada seja corrigida em uma unica passada.
func (inv ComponentInventory) Validate(component string) error {
	var errs []error

	if inv.Component != "" && inv.Component != component {
		errs = append(errs, fmt.Errorf("componente declarado como %q mas carregado como %q", inv.Component, component))
	}

	errs = append(errs, validateFallbacks(inv.Fallbacks)...)

	for i, c := range inv.Conversations {
		errs = append(errs, validateConversation(c, fmt.Sprintf("conversations[%d]", i))...)

		if c.Eligibility != nil && !slices.Contains(knownSources, c.Source()) {
			errs = append(errs, fmt.Errorf("conversa %q: source desconhecida %q", c.ID, c.Source()))
		}
	}

	errs = append(errs, validateUniqueIDs(inv)...)

	return errors.Join(errs...)
}

func validateFallbacks(fallbacks []Conversation) []error {
	var errs []error

	seen := make(map[string]struct{}, len(fallbacks))
	hasDefault := false

	for i, f := range fallbacks {
		errs = append(errs, validateConversation(f, fmt.Sprintf("fallbacks[%d]", i))...)

		if f.Eligibility != nil {
			errs = append(errs, fmt.Errorf("fallback %q: nao pode declarar eligibility", f.ID))
		}
		if _, dup := seen[f.Product]; dup {
			errs = append(errs, fmt.Errorf("fallback %q: ja existe outro fallback para o produto %q", f.ID, f.Product))
		}
		seen[f.Product] = struct{}{}

		if f.Product == DefaultFallbackProduct {
			hasDefault = true
		}
	}

	if !hasDefault {
		errs = append(errs, errors.New("fallback default (product vazio) e obrigatorio"))
	}
	return errs
}

func validateConversation(c Conversation, position string) []error {
	var errs []error

	label := position
	if c.ID != "" {
		label = fmt.Sprintf("%s (%s)", position, c.ID)
	}

	if c.ID == "" {
		errs = append(errs, fmt.Errorf("%s: id e obrigatorio", label))
	}
	if c.Text == "" {
		errs = append(errs, fmt.Errorf("%s: text e obrigatorio", label))
	}
	if c.Link == "" {
		errs = append(errs, fmt.Errorf("%s: link e obrigatorio", label))
	}
	if !slices.Contains(knownTypes, c.Type) {
		errs = append(errs, fmt.Errorf("%s: type invalido %q", label, c.Type))
	}
	return errs
}

func validateUniqueIDs(inv ComponentInventory) []error {
	var errs []error

	seen := make(map[string]struct{}, len(inv.Conversations)+len(inv.Fallbacks))
	for _, c := range slices.Concat(inv.Fallbacks, inv.Conversations) {
		if c.ID == "" {
			continue
		}
		if _, dup := seen[c.ID]; dup {
			errs = append(errs, fmt.Errorf("id duplicado no componente: %q", c.ID))
		}
		seen[c.ID] = struct{}{}
	}
	return errs
}

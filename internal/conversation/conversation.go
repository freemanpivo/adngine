package conversation

type Type string

const (
	TypeKnowledge  Type = "knowledge"
	TypeAction     Type = "action"
	TypeEvaluation Type = "evaluation"
)

type Source string

const (
	SourceStatic   Source = "static"
	SourceDynamoDB Source = "dynamodb"
	SourceHTTP     Source = "http"
)

// Rule e Request ficam como mapas crus: o inventario apenas transporta a
// especificacao, e o motor de elegibilidade a compila.
type EligibilitySpec struct {
	Source  Source         `mapstructure:"source" yaml:"source"`
	Request map[string]any `mapstructure:"request" yaml:"request"`
	Rule    map[string]any `mapstructure:"rule" yaml:"rule"`
}

type Conversation struct {
	ID          string           `mapstructure:"id" yaml:"id"`
	Type        Type             `mapstructure:"type" yaml:"type"`
	Product     string           `mapstructure:"product" yaml:"product"`
	Text        string           `mapstructure:"text" yaml:"text"`
	Link        string           `mapstructure:"link" yaml:"link"`
	Priority    int              `mapstructure:"priority" yaml:"priority"`
	Eligibility *EligibilitySpec `mapstructure:"eligibility" yaml:"eligibility"`
}

func (c Conversation) Source() Source {
	if c.Eligibility == nil || c.Eligibility.Source == "" {
		return SourceStatic
	}
	return c.Eligibility.Source
}

// ComponentInventory e o conteudo de um arquivo de componente. O fallback de
// produto vazio e o default e e obrigatorio.
type ComponentInventory struct {
	Component     string         `mapstructure:"component" yaml:"component"`
	Fallbacks     []Conversation `mapstructure:"fallbacks" yaml:"fallbacks"`
	Conversations []Conversation `mapstructure:"conversations" yaml:"conversations"`
}

type Client struct {
	ID      string
	Product string
}

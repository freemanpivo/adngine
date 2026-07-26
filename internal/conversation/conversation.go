package conversation

type Type string

const (
	TypeKnowledge  Type = "knowledge"
	TypeAction     Type = "action"
	TypeEvaluation Type = "evaluation"
)

type Conversation struct {
	ID         string   `mapstructure:"id" yaml:"id"`
	Type       Type     `mapstructure:"type" yaml:"type"`
	Product    string   `mapstructure:"product" yaml:"product"`
	Text       string   `mapstructure:"text" yaml:"text"`
	Link       string   `mapstructure:"link" yaml:"link"`
	Priority   int      `mapstructure:"priority" yaml:"priority"`
	Components []string `mapstructure:"components" yaml:"components"`
}

type Client struct {
	ID      string
	Product string
}

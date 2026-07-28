package eligibility

// MaxActualLen limita o valor observado que vai para o log (RF-36).
const MaxActualLen = 64

// Collector recebe o caminho percorrido pela avaliacao. Enabled permite que o
// avaliador pule o trabalho que so existe para o trace: com o coletor nulo,
// nada e alocado no hot path (RF-37).
type Collector interface {
	Enabled() bool
	Predicate(p *Predicate, matched, found bool, actual any)
}

// Nop e o coletor default. Nao guarda nada e nao aloca.
var Nop Collector = nopCollector{}

type nopCollector struct{}

func (nopCollector) Enabled() bool                         { return false }
func (nopCollector) Predicate(*Predicate, bool, bool, any) {}

type PredicateTrace struct {
	PredicateID string `json:"predicate_id"`
	Label       string `json:"label"`
	Matched     bool   `json:"matched"`
	Found       bool   `json:"found"`
	Actual      string `json:"actual,omitempty"`
}

// Recorder grava os predicados na ordem de avaliacao.
type Recorder struct {
	Predicates []PredicateTrace
}

func NewRecorder() *Recorder { return &Recorder{} }

func (r *Recorder) Enabled() bool { return true }

func (r *Recorder) Predicate(p *Predicate, matched, found bool, actual any) {
	entry := PredicateTrace{
		PredicateID: p.ID,
		Label:       p.Label,
		Matched:     matched,
		Found:       found,
	}
	if found && p.LogValue {
		entry.Actual = truncate(renderValue(actual))
	}
	r.Predicates = append(r.Predicates, entry)
}

func truncate(s string) string {
	if len(s) <= MaxActualLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= MaxActualLen {
		return s
	}
	return string(runes[:MaxActualLen])
}

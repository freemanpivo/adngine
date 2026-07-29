package eligibility

import "slices"

type Result struct {
	Eligible  bool
	Adherence float64

	// FirstFailed e o primeiro predicado required reprovado na ordem de
	// avaliacao, preenchido apenas quando a conversa nao e elegivel (RF-34).
	FirstFailed *Predicate

	// FieldsRead so e preenchido com o coletor ativo: denuncia regra apontando
	// para chave inexistente na bag.
	FieldsRead []string
}

// Evaluate percorre a arvore inteira, sem curto-circuito: aderencia e trace
// precisam do resultado de todos os predicados, nao apenas dos que decidiriam a
// elegibilidade.
func Evaluate(bag map[string]any, rule *CompiledRule, collector Collector) Result {
	if collector == nil {
		collector = Nop
	}
	if rule == nil {
		return Result{Eligible: true, Adherence: 1}
	}

	e := evaluator{bag: bag, collector: collector, trace: collector.Enabled()}
	pass, contributes := e.eval(&rule.Root, false)
	eligible := pass || !contributes

	result := Result{
		Eligible:   eligible,
		Adherence:  adherence(e.satisfiedWeight, rule.TotalWeight),
		FieldsRead: e.fieldsRead,
	}
	if !eligible {
		result.FirstFailed = e.firstFailed
	}
	return result
}

type evaluator struct {
	bag             map[string]any
	collector       Collector
	trace           bool
	satisfiedWeight float64
	firstFailed     *Predicate
	fieldsRead      []string
}

// eval devolve, alem do resultado do no, se ele participa da elegibilidade.
// Predicados required: false sao transparentes em qualquer profundidade - eles
// so alimentam a aderencia (RF-12).
func (e *evaluator) eval(rule *Rule, negated bool) (pass, contributes bool) {
	switch rule.Kind {
	case KindPredicate:
		return e.evalPredicate(rule.Predicate, negated)

	case KindNot:
		childPass, childContributes := e.eval(&rule.Children[0], !negated)
		return !childPass, childContributes

	case KindAny:
		pass = false
		for i := range rule.Children {
			childPass, childContributes := e.eval(&rule.Children[i], negated)
			if !childContributes {
				continue
			}
			contributes = true
			pass = pass || childPass
		}
		return pass, contributes

	default:
		pass = true
		for i := range rule.Children {
			childPass, childContributes := e.eval(&rule.Children[i], negated)
			if !childContributes {
				continue
			}
			contributes = true
			pass = pass && childPass
		}
		return pass, contributes
	}
}

func (e *evaluator) evalPredicate(p *Predicate, negated bool) (pass, contributes bool) {
	actual, found := Lookup(e.bag, p.Field)

	matched := false
	if op, ok := operators[p.Op]; ok {
		matched = op(actual, found, p)
	}

	if e.trace {
		e.collector.Predicate(p, matched, found, actual)
		if !slices.Contains(e.fieldsRead, p.Field) {
			e.fieldsRead = append(e.fieldsRead, p.Field)
		}
	}

	// Dentro de um not, satisfazer o predicado e justamente nao dar match.
	if matched != negated {
		e.satisfiedWeight += p.Weight
	} else if p.Required && e.firstFailed == nil {
		e.firstFailed = p
	}

	return matched, p.Required
}

func adherence(satisfied, total float64) float64 {
	if total <= 0 {
		return 1
	}
	return satisfied / total
}

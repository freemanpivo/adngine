package eligibility

// ValidateRule roda a compilacao apenas pelos erros: operador desconhecido,
// regex invalida, in/not_in sem lista, field vazio (RF-14). Serve ao fail fast
// da carga do inventario, que descarta a regra compilada.
func ValidateRule(raw map[string]any) error {
	_, err := Compile("", raw)
	return err
}

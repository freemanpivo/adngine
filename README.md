# adngine

API de selecao de anuncios. Voce cadastra **conversas** (pecas de conteudo publicitario) associadas a um tipo
(`knowledge`, `action`, `evaluation`), opcionalmente a um produto (`veiculo`, `eletrodomestico`, etc.), a uma
prioridade e aos componentes de tela onde podem aparecer (`banner`, `card`, `footer`). Dado um cliente e os
componentes solicitados, a API retorna a melhor conversa para cada um.

O cadastro de conversas atualmente e feito via arquivo (`configs/conversations.yaml`).

## Rodando localmente

```bash
go run ./cmd/adngine
```

O servidor sobe na porta configurada em `configs/config.yaml` (padrao `8080`).

## Testando com curl

```bash
curl -X POST http://localhost:8080/v1/selections \
  -H 'Content-Type: application/json' \
  -d '{
    "client": {
      "id": "cliente-123",
      "product": "veiculo"
    },
    "slots": ["banner", "card", "footer"]
  }'
```

Resposta esperada (uma conversa por slot, ou `null` se nao houver nenhuma elegivel):

```json
{
  "selections": {
    "banner": {
      "id": "conv-veiculos-promo",
      "type": "action",
      "product": "veiculo",
      "text": "Confira as ofertas da semana em veiculos",
      "link": "https://example.com/veiculos/promocoes",
      "priority": 10
    },
    "card": { "...": "..." },
    "footer": { "...": "..." }
  }
}
```

Sem contexto de produto (`product` omitido), so conversas sem produto associado sao elegiveis:

```bash
curl -X POST http://localhost:8080/v1/selections \
  -H 'Content-Type: application/json' \
  -d '{
    "client": { "id": "cliente-456" },
    "slots": ["banner", "card", "footer"]
  }'
```

## Configuracao

- `configs/config.yaml` - porta do servidor, nivel de log e caminho do arquivo de conversas.
- `configs/conversations.yaml` - registro das conversas disponiveis.

Para usar um arquivo de config diferente:

```bash
go run ./cmd/adngine -config path/to/config.yaml
```

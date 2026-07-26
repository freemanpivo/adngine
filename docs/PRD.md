# PRD - adngine v1

Status: proposta
Data: 2026-07-26
Autor: time adngine

## 1. Contexto

A versao atual do adngine e um scaffold. Ele prova o fluxo `HTTP -> Registry -> Selector -> Repository`,
mas assume que:

- todas as conversas vivem em um unico arquivo (`configs/conversations.yaml`), com uma lista `components`
  dentro de cada conversa;
- elegibilidade e apenas "produto da conversa igual ao produto do cliente";
- nao existe nenhuma integracao externa, portanto nao existe timeout, fallback nem paralelismo;
- a resposta pode ser `null` para um slot;
- nao ha como explicar por que uma conversa foi ou nao escolhida.

Nenhuma dessas premissas sobrevive ao produto real. Este documento descreve a v1: inventario por componente,
motor de elegibilidade plugavel (DynamoDB e API), contrato de latencia com fallback garantido e rastreabilidade
da decisao.

## 2. Objetivos

1. Separar o inventario de conversas por componente, para que cada componente possa ser operado e alterado
   de forma independente.
2. Substituir o filtro fixo de produto por um motor de elegibilidade orientado a regras, capaz de consumir
   dados de origens com schema desconhecido.
3. Suportar duas origens de elegibilidade: DynamoDB (PK = id do cliente) e API HTTP.
4. Deixar a origem HTTP disponivel para qualquer componente. Usar API e um tradeoff de latencia, nao uma
   restricao de arquitetura: o componente que a usa precisa de um orcamento de tempo maior. Na v1 apenas o
   `footer` faz uso dela.
5. Garantir contrato de latencia: `footer` responde em ate 500ms, a requisicao inteira em ate 1s, sempre com
   conteudo (fallback quando necessario).
6. Tornar a decisao do motor auditavel: para qualquer requisicao e possivel reconstruir, pelos logs, por que
   cada conversa foi descartada e por que a vencedora ganhou.

### Nao-objetivos (v1)

- Painel de administracao ou CRUD de conversas via API. O cadastro continua em arquivo de configuracao.
- Persistencia das conversas em banco de dados.
- Recarga a quente do inventario (`SIGHUP` ou watcher de arquivo). Fica para a v2; o `RWMutex` do repositorio
  ja prepara o terreno.
- Metricas de performance publicitaria (impressao, clique, conversao).
- Personalizacao por machine learning ou ranking probabilistico.
- Multi-tenancy.

## 3. Metricas de sucesso

| Metrica | Alvo |
| --- | --- |
| p99 de latencia do `POST /v1/selections` | <= 1s (hard cap) |
| p99 de latencia do slot `footer` | <= 500ms (hard cap) |
| Taxa de resposta 5xx causada por dependencia externa | 0% |
| Taxa de slots preenchidos (conversa ou fallback) | 100% |
| Taxa de fallback em condicao normal (dependencias saudaveis) | < 2% |
| Cobertura de testes no motor de elegibilidade | >= 90% |
| Decisoes reconstruiveis a partir do log com trace ligado | 100% |
| Overhead de latencia do trace desligado | ~0 (sem alocacao no hot path) |

## 4. Conceitos

- **Conversa**: peca de conteudo publicitario (`id`, `type`, `product`, `text`, `link`, `priority`).
- **Componente / slot**: espaco de exibicao (`banner`, `card`, `footer`).
- **Inventario**: conjunto de conversas cadastradas para um componente.
- **Bag de atributos**: mapa `chave -> valor` com os dados do cliente vindos de uma origem externa. Nao tem
  schema fixo; o formato depende da origem.
- **Regra de elegibilidade**: expressao declarativa avaliada contra a bag de atributos. O resultado e binario:
  passou ou nao passou.
- **Aderencia**: nota de 0 a 1 que mede quantos predicados (ponderados) a bag satisfez. Nao e criterio de
  corte; serve como desempate no ranking e como sinal de diagnostico.
- **Fallback**: conversa de reserva declarada no arquivo do componente, uma por produto, servida quando nao ha
  conversa elegivel, quando a origem falha ou quando o orcamento de tempo acaba.
- **Trace de decisao**: registro estruturado do caminho percorrido pelo motor em uma requisicao - candidatos
  avaliados, predicado a predicado, com o motivo de exclusao de cada um e o criterio que elegeu o vencedor.

## 5. Requisitos funcionais

### 5.1 Inventario por componente

**RF-01** - O inventario passa a ser um arquivo por componente:

```
configs/
  config.yaml
  conversations/
    banner.yaml
    card.yaml
    footer.yaml
```

**RF-02** - O campo `components` deixa de existir dentro da conversa. O componente e definido pelo arquivo.
Uma conversa que deve aparecer em dois componentes e declarada nos dois arquivos. A duplicacao e intencional:
o inventario de cada componente e independente e pode divergir (texto, prioridade, regra). Nao ha deduplicacao
entre componentes - se a mesma conversa for elegivel em dois slots, ela e exibida nos dois.

**RF-03** - Cada arquivo declara uma lista `fallbacks`, um por produto. O fallback tem os mesmos campos de uma
conversa normal (`id`, `type`, `product`, `text`, `link`, `priority`), exceto `eligibility`, que nao se aplica.
E obrigatorio existir um fallback default (`product` vazio) por componente; a aplicacao nao sobe sem ele.

**RF-04** - Escolha do fallback: o de `product` igual a `client.product`; se nao houver, o default. Cliente sem
contexto de produto recebe sempre o default.

**RF-05** - `id` deve ser unico dentro de um arquivo, considerando conversas e fallbacks juntos. Tambem nao
pode haver dois fallbacks para o mesmo produto. A aplicacao nao sobe nesses casos.

Formato do arquivo de componente:

```yaml
component: footer

fallbacks:
  - product: ""
    id: fb-footer-default
    type: knowledge
    text: "Saiba mais sobre nossa empresa"
    link: "https://example.com/sobre"
    priority: 0
  - product: veiculo
    id: fb-footer-veiculo
    type: knowledge
    text: "Conheca nosso catalogo de veiculos"
    link: "https://example.com/veiculos"
    priority: 0

conversations:
  - id: conv-footer-credito
    type: action
    product: veiculo
    text: "Credito pre-aprovado para o seu proximo carro"
    link: "https://example.com/credito"
    priority: 10
    eligibility:
      source: http
      request:
        endpoint: /v1/credit-profile
        method: GET
        query:
          client_id: "{{client.id}}"
      rule:
        all:
          - field: perfil.faixa_renda
            op: in
            value: [A, B]
            weight: 2
          - field: possui_veiculo
            op: eq
            value: false
            weight: 1
            required: false
```

### 5.2 Motor de elegibilidade

**RF-06** - Cada conversa tem um bloco `eligibility` com uma `source`:

| source | Descricao | Custo tipico |
| --- | --- | --- |
| `static` | Sem consulta externa. Mantem o comportamento atual (filtro de produto). | ~0 |
| `dynamodb` | `GetItem` por `idcliente`. O item vira a bag de atributos. | uma leitura por requisicao |
| `http` | Chamada a uma API externa. O corpo JSON vira a bag de atributos. | uma chamada de rede por endpoint distinto |

Qualquer componente pode usar qualquer source. A escolha de `http` e uma decisao de tradeoff, nao uma
restricao tecnica: o componente que a adota precisa de um orcamento de tempo compativel (ver RF-20 e RF-28).
`eligibility` ausente equivale a `source: static` sem regra.

**RF-07** - O filtro de produto continua sendo aplicado antes das regras, para todas as sources: uma conversa
com `product` preenchido so e candidata se `client.product` for igual. Conversa sem `product` e sempre
candidata.

**RF-08** - A regra e uma arvore de combinadores e predicados:

- Combinadores: `all` (E), `any` (OU), `not`.
- Predicado: `{ field, op, value, weight, required, log_value }`.
- `field` usa caminho com ponto para mapas aninhados e indice para listas: `perfil.enderecos[0].uf`.
- `weight` default `1`. `required` default `true`. `log_value` default `true` (ver RF-35).

**RF-09** - Operadores minimos da v1: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `in`, `not_in`, `contains`,
`exists`, `not_exists`, `truthy`, `regex`.

**RF-10** - Comparacao tolerante a tipo. Numero vindo do DynamoDB (`N`, string) e numero vindo de JSON
(`float64`) devem comparar igual a `700` no YAML. Strings de booleano (`"S"`, `"true"`, `"1"`) sao aceitas
por `truthy`. Campo ausente nunca causa erro: os predicados falham, exceto `not_exists`.

**RF-11** - Elegibilidade e binaria: a conversa e elegivel quando todos os predicados `required: true` sao
satisfeitos. Nao existe corte por nota. Conversa nao elegivel e simplesmente descartada e o motor segue para a
proxima candidata.

**RF-12** - Aderencia: `soma(weight dos predicados satisfeitos) / soma(weight de todos os predicados)`.
Regra sem predicados tem aderencia `1.0`. Predicados `required: false` existem exclusivamente para alimentar
essa nota - eles nunca desqualificam uma conversa, apenas a posicionam melhor ou pior no ranking.

**RF-13** - Ordenacao entre elegiveis: `priority` desc -> `adherence` desc -> `id` asc. O criterio de
desempate por `id` existe para tornar a selecao deterministica.

**RF-14** - Validacao na inicializacao (fail fast): operador desconhecido, source desconhecida, `regex`
invalida, `field` vazio, `in`/`not_in` sem lista, referencia a endpoint HTTP nao configurado, fallback default
ausente, ids duplicados.

### 5.3 Origem DynamoDB

**RF-15** - Uma unica leitura (`GetItem`) por requisicao, com PK `idcliente = client.id`. O resultado e
memoizado no escopo da requisicao e compartilhado por todos os slots e conversas que usam `source: dynamodb`.

**RF-16** - O item retornado e convertido para `map[string]any` sem schema declarado: `S -> string`,
`N -> float64`, `BOOL -> bool`, `L -> []any`, `M -> map[string]any`, `NULL -> nil`. Novas colunas passam a
ser utilizaveis em regras sem alteracao de codigo.

**RF-17** - Leitura eventualmente consistente (`ConsistentRead: false`).

**RF-18** - Cliente inexistente devolve bag vazia. Nao e erro: regras simplesmente falham e o fallback entra.

**RF-19** - Erro ou timeout do DynamoDB nao derruba a requisicao. Os slots que dependem dele caem para o
fallback com `reason: provider_error` ou `reason: timeout`.

### 5.4 Origem HTTP

**RF-20** - `source: http` e permitido em qualquer componente. O que muda entre componentes e o orcamento: o
timeout efetivo da chamada e `min(eligibility.http.timeout, orcamento restante do componente)`. Um componente
com orcamento de 200ms que declare uma conversa com `source: http` de 500ms vai cair em `timeout` de forma
sistematica - por isso a inicializacao emite **aviso** (nao erro) quando o orcamento do componente e menor que
o timeout HTTP configurado, nomeando componente e conversa. A decisao de assumir o custo e do time de produto.

**RF-21** - O bloco `request` define `endpoint`, `method`, `query`, `headers` e `body`, com interpolacao
limitada a variaveis conhecidas: `{{client.id}}`, `{{client.product}}` e `{{client.attributes.<chave>}}`.
Qualquer outra variavel e erro de inicializacao.

**RF-22** - Chamadas com o mesmo endpoint e os mesmos parametros resolvidos sao deduplicadas dentro da
requisicao (uma chamada, resultado compartilhado), inclusive entre componentes diferentes.

**RF-23** - Limite de chamadas HTTP distintas por componente configuravel (`max_calls`, default `3`),
validado na inicializacao contra o inventario. As chamadas de um componente ocorrem em paralelo dentro do seu
orcamento.

**RF-24** - Sem retry. O orcamento e curto demais para uma segunda tentativa segura.

**RF-25** - Circuit breaker por endpoint. Aberto, o endpoint falha imediatamente sem consumir orcamento e as
conversas dependentes sao descartadas (nao sao elegiveis), sobrando as demais candidatas do componente ou o
fallback.

**RF-26** - Cache opcional em memoria com TTL curto (default desligado), com chave = endpoint + parametros
resolvidos.

### 5.5 Orcamento de tempo e fallback

**RF-27** - O handler cria um contexto com deadline global (`selection.global_timeout`, default `1s`) no
inicio da requisicao.

**RF-28** - Cada componente tem seu proprio orcamento (`timeout`), sempre limitado pelo tempo restante do
deadline global. Defaults: `banner` 200ms, `card` 200ms, `footer` 500ms. O orcamento e o unico mecanismo que
diferencia um componente "barato" de um componente que fala com API.

**RF-29** - Os slots sao resolvidos em paralelo, um por goroutine. A falha ou o estouro de um slot nao afeta
os demais.

**RF-30** - Ao atingir o deadline global, os slots ainda nao concluidos sao respondidos com o fallback do seu
componente. A resposta e enviada; goroutines pendentes sao canceladas via contexto.

**RF-31** - Fallback e servido em quatro situacoes, sempre com `reason` explicito:

| reason | Situacao |
| --- | --- |
| `no_eligible` | Nenhuma conversa passou nas regras |
| `timeout` | Orcamento do componente ou deadline global estourou |
| `provider_error` | DynamoDB ou API retornou erro, ou o breaker estava aberto |
| `unknown_component` | Slot solicitado nao existe no registry |

**RF-32** - A API nunca retorna 5xx por falha de dependencia externa. 5xx fica reservado a bug interno.

**RF-33** - `unknown_component`: slot desconhecido nao tem fallback configurado. Retorna entrada com
`conversation: null` e `reason: unknown_component`, sem falhar a requisicao.

### 5.6 Rastreabilidade da decisao

**RF-34** - O motor produz um trace por slot com, no minimo:

- `component`, `source`, `client_id`, `elapsed_ms`;
- lista de candidatos considerados, na ordem final de ranking, cada um com `conversation_id`, `outcome`
  (`selected`, `eligible_not_ranked`, `rejected_product`, `rejected_predicate`, `rejected_provider_error`,
  `rejected_breaker_open`), `adherence` e `priority`;
- para os rejeitados por regra, o `predicate_id` e o `label` do **primeiro** predicado `required` que falhou;
- para o vencedor, o criterio que decidiu o desempate (`priority`, `adherence` ou `id`);
- as chaves da bag efetivamente lidas (`fields_read`), o que denuncia rapidamente regra apontando para coluna
  inexistente.

**RF-35** - Campos pre-computados no carregamento do inventario, para que o trace nao pague formatacao de
string no hot path:

- `predicate_id` estavel por predicado (`<conversation_id>#<indice>`);
- `label` textual ja renderizado (`perfil.faixa_renda in [A,B]`);
- soma total dos pesos por conversa (denominador da aderencia);
- lista plana dos predicados de cada conversa, usada apenas pelo trace (a avaliacao continua percorrendo a
  arvore);
- `regexp` ja compilada.

**RF-36** - O valor observado (`actual`) so vai para o log quando `log_value: true` no predicado, e sempre
truncado em 64 caracteres. Campos sensiveis da bag sao marcados com `log_value: false`. O valor esperado
(`value`) vem da configuracao e sempre pode ser logado.

**RF-37** - Ativacao do trace via `log.decision_trace`: `off` (default), `sampled` (com
`log.decision_trace_sample_rate`, default `0.01`) ou `on`. Desligado, nenhuma estrutura de trace e alocada -
o avaliador recebe um coletor nulo. Independentemente do modo, um log resumido por slot e sempre emitido em
nivel `info`: componente, vencedor, `fallback`, `reason`, quantidade de candidatos avaliados e `elapsed_ms`.

**RF-38** - Um request pode pedir o trace com `debug: true` no corpo, mas ele so e devolvido na resposta
(campo `trace`) quando `selection.debug.expose_trace` estiver habilitado na configuracao, o que nao e o caso
em producao. Com a flag desabilitada, `debug: true` apenas forca o trace no log daquela requisicao.

### 5.7 Contrato HTTP

**RF-39** - Request ganha `client.attributes`, mapa opcional de atributos fornecidos pelo chamador. Eles
entram na bag com a menor precedencia (origem externa sobrescreve) e podem ser usados na interpolacao do
`request` HTTP.

```json
{
  "client": {
    "id": "cliente-123",
    "product": "veiculo",
    "attributes": { "canal": "app", "segmento": "varejo" }
  },
  "slots": ["banner", "card", "footer"],
  "debug": false
}
```

**RF-40** - Response passa a expor metadados por slot. Mudanca breaking em relacao ao formato atual,
aceitavel porque nao ha consumidor em producao.

```json
{
  "selections": {
    "banner": {
      "conversation": {
        "id": "conv-veiculos-promo",
        "type": "action",
        "product": "veiculo",
        "text": "Confira as ofertas da semana em veiculos",
        "link": "https://example.com/veiculos/promocoes",
        "priority": 10
      },
      "fallback": false,
      "reason": null,
      "adherence": 0.83,
      "elapsed_ms": 12
    },
    "footer": {
      "conversation": { "id": "fb-footer-veiculo", "...": "..." },
      "fallback": true,
      "reason": "timeout",
      "adherence": null,
      "elapsed_ms": 500
    }
  }
}
```

**RF-41** - `GET /health` (liveness) e `GET /ready` (readiness: config carregada e inventario valido).

## 6. Requisitos nao funcionais

- **RNF-01** - Deadline global e orcamentos por componente configuraveis, sem recompilar.
- **RNF-02** - Cliente HTTP unico e reutilizado, com pool de conexoes e keep-alive dimensionados
  (`max_idle_conns`, `max_conns_per_host`).
- **RNF-03** - Log estruturado (`log/slog`) com `request_id`, `client_id`, `component`, `source`,
  `elapsed_ms`, `reason`. Textos de conversa nao vao para o log.
- **RNF-04** - Metricas Prometheus: latencia por componente, taxa de fallback por `reason`, latencia e erro
  por origem, estado do circuit breaker, contador de rejeicao por predicado.
- **RNF-05** - Inventario carregado uma vez na inicializacao e mantido em memoria, protegido por `RWMutex`
  para permitir recarga futura.
- **RNF-06** - O motor de regras e puro (sem I/O) e testavel isoladamente, incluindo a geracao de trace.
- **RNF-07** - Credenciais AWS via cadeia padrao do SDK. Nada de segredo em arquivo de configuracao.
- **RNF-08** - Shutdown gracioso com drenagem das requisicoes em voo.

## 7. Arquitetura alvo

```
cmd/adngine
  -> internal/app            wiring
    -> internal/config       config tipada (timeouts, origens, arquivos por componente, trace)
    -> internal/httpserver   rotas, DTOs, orquestracao de slots e deadline global
      -> internal/selection  Registry, Selector por componente, ranking, fallback por produto
        -> internal/eligibility  motor de regras (puro) + trace + resolvers de bag
          -> internal/provider/dynamodb
          -> internal/provider/httpapi
        -> internal/conversation  modelo + Repository por componente
```

Pacotes novos:

- `internal/eligibility` - `Rule`, `Predicate`, `Evaluate(bag, rule, collector) Result`, resolucao de `field`
  path, coercao de tipos, `Trace`/`Collector`.
- `internal/provider` - interface `AttributeProvider` com `Fetch(ctx, client) (map[string]any, error)`,
  implementacoes `static`, `dynamodb`, `httpapi`, e o memoizador por requisicao.
- `internal/selection/resolver.go` - resolve um componente dentro do seu orcamento e devolve
  `Result{Conversation, Fallback, Reason, Adherence, Trace}`.

## 8. Epicos e tarefas

Cada tarefa e independente o suficiente para virar um item de backlog. Dependencias explicitas entre colchetes.

### Epico 0 - Rede de seguranca

**T-001 - Testes do comportamento atual**
Cobrir `bestMatch`, `Repository.ByComponent` e o handler com o inventario atual.
Aceite: `go test ./...` verde; testes falham se o ranking por prioridade ou o filtro de produto mudar.
Arquivos: `internal/selection/selector_test.go`, `internal/conversation/repository_test.go`,
`internal/httpserver/handler_test.go`.

**T-002 - Fixtures de teste**
Diretorio `testdata` com inventarios de componente validos e invalidos, reaproveitado pelos demais epicos.
Aceite: fixtures carregam via helper unico. [T-001]

### Epico 1 - Inventario por componente

**T-101 - Modelo de arquivo de componente**
Novos tipos `ComponentInventory{Component, Fallbacks, Conversations}`. Remover `Components []string` de
`Conversation`. Adicionar `Eligibility *EligibilitySpec` (ainda nao avaliado).
Aceite: `go build ./...` limpo; campo `components` no YAML antigo passa a ser ignorado com aviso.
Arquivos: `internal/conversation/conversation.go`.

**T-102 - Repositorio por componente**
`Repository.LoadComponent(component, path)`, `Candidates(component)` e `Fallback(component, product)`
implementando a escolha da RF-04. `ByComponent` sai.
Aceite: carregar 3 arquivos produz 3 inventarios isolados; componente desconhecido devolve vazio; produto sem
fallback proprio cai no default. [T-101]
Arquivos: `internal/conversation/repository.go`.

**T-103 - Validacao de inventario na inicializacao**
Ids duplicados no mesmo arquivo (conversas + fallbacks), dois fallbacks para o mesmo produto, fallback default
ausente, `component` divergente do esperado, campos obrigatorios vazios (`id`, `text`, `link`).
Aceite: cada violacao produz erro nomeando arquivo e id; a aplicacao nao sobe. [T-102]
Arquivos: `internal/conversation/validate.go`.

**T-104 - Config de componentes**
`selection.components.<nome>.{file_path,timeout,max_calls}` e `selection.global_timeout` na config tipada,
com defaults (200ms banner, 200ms card, 500ms footer, 1s global, `max_calls: 3`).
Aceite: config sem o bloco usa defaults; `timeout` de componente maior que o global e erro. [T-102]
Arquivos: `internal/config/config.go`, `configs/config.yaml`.

**T-105 - Quebrar o YAML atual em tres arquivos**
Gerar `configs/conversations/{banner,card,footer}.yaml` a partir do inventario atual, com a lista de
`fallbacks` por produto (default + `veiculo` + `eletrodomestico`). Remover `configs/conversations.yaml`.
Aceite: mesmas conversas de hoje disponiveis por componente; `configs/conversations.yaml` deixa de existir.
[T-103]

**T-106 - Wiring**
`app.New` carrega N inventarios e monta o `Registry` a partir dos componentes configurados, em vez do mapa
fixo de tres selectors.
Aceite: adicionar um componente novo passa a exigir apenas config + arquivo, salvo logica propria. [T-104, T-105]
Arquivos: `internal/app/app.go`, `internal/selection/selector.go`.

### Epico 2 - Motor de elegibilidade (puro)

**T-201 - Tipos da regra**
`Rule` (`All`, `Any`, `Not`, `Predicate`), `Predicate{Field, Op, Value, Weight, Required, LogValue}` com decode
do YAML e defaults (`weight: 1`, `required: true`, `log_value: true`).
Aceite: round-trip YAML -> struct para regras aninhadas de 3 niveis.
Arquivos: `internal/eligibility/rule.go`.

**T-202 - Pre-computacao no carregamento**
Gerar `predicate_id`, `label` textual, soma total de pesos por conversa, lista plana de predicados e regex
compilada (RF-35), tudo uma vez, na carga.
Aceite: benchmark mostra zero alocacao de string por avaliacao; `label` estavel entre reinicios. [T-201]
Arquivos: `internal/eligibility/compile.go`.

**T-203 - Resolucao de caminho de campo**
`Lookup(bag, "a.b[0].c") (any, bool)` sobre `map[string]any` e `[]any`.
Aceite: casos de mapa aninhado, indice de lista, indice fora do range, chave ausente, caminho vazio. [T-201]
Arquivos: `internal/eligibility/path.go`.

**T-204 - Coercao e comparacao de tipos**
Comparadores tolerantes entre `string`, `float64`, `int`, `bool` e as representacoes textuais de numero e
booleano vindas do DynamoDB.
Aceite: `"700" == 700`, `"S"` e truthy, `"false"` nao e truthy, tipos incomparaveis devolvem falso sem panic.
[T-203]
Arquivos: `internal/eligibility/coerce.go`.

**T-205 - Operadores**
Implementar os 13 operadores da RF-09 com registry de operadores, para extensao futura.
Aceite: tabela de testes cobrindo cada operador com hit, miss e campo ausente. [T-204]
Arquivos: `internal/eligibility/ops.go`.

**T-206 - Avaliador**
`Evaluate(bag, rule, collector) Result{Eligible, Adherence, FirstFailedPredicate, FieldsRead}` conforme RF-11
e RF-12, incluindo semantica de peso dentro de `any` e `not`. Sem corte por nota.
Aceite: regra vazia devolve `{true, 1.0}`; predicado `required` falho zera a elegibilidade mas a aderencia
continua sendo calculada; predicado `required: false` nunca desqualifica. [T-205]
Arquivos: `internal/eligibility/evaluate.go`.

**T-207 - Coletor de trace**
Interface `Collector` com implementacao nula (default) e implementacao de gravacao, alimentada pelos campos
pre-computados de T-202. Registra por predicado: `predicate_id`, `label`, `matched`, `actual` (respeitando
`log_value` e truncamento de 64 chars).
Aceite: com coletor nulo, benchmark identico ao de T-206 sem trace; com coletor ativo, trace contem todos os
predicados avaliados na ordem de avaliacao. [T-206, T-202]
Arquivos: `internal/eligibility/trace.go`.

**T-208 - Validacao estatica de regra**
`ValidateRule(rule) error`: operador desconhecido, regex invalida, `in`/`not_in` sem lista, `field` vazio.
Aceite: chamada durante o carregamento do inventario; erro cita o id da conversa. [T-206, T-103]
Arquivos: `internal/eligibility/validate.go`.

### Epico 3 - Abstracao de origem

**T-301 - Interface AttributeProvider**
`Fetch(ctx, conversation.Client) (map[string]any, error)` e o registry `source -> provider`.
Aceite: provider `static` devolve bag apenas com `client.attributes`.
Arquivos: `internal/provider/provider.go`, `internal/provider/static.go`.

**T-302 - Memoizacao por requisicao**
Resolver com escopo de requisicao que garante uma unica busca por chave de origem, com deduplicacao de
chamadas concorrentes (`singleflight`), valendo tambem entre componentes distintos.
Aceite: dois slots com a mesma origem produzem uma chamada; teste concorrente com `-race`. [T-301]
Arquivos: `internal/provider/resolver.go`.

**T-303 - Selector usando o motor**
`bestMatch` passa a: filtrar por produto -> buscar bag -> avaliar regra -> ordenar por
`priority/adherence/id`, alimentando o coletor de trace a cada etapa (inclusive nas rejeicoes por produto).
Aceite: inventario sem `eligibility` produz exatamente o mesmo resultado de T-001. [T-302, T-207]
Arquivos: `internal/selection/selector.go`.

### Epico 4 - Origem DynamoDB

**T-401 - Dependencia e cliente AWS**
Adicionar `aws-sdk-go-v2` (config, dynamodb, attributevalue), construir o cliente uma vez no wiring.
Aceite: `go mod tidy` limpo; cliente injetado, nao global.
Arquivos: `go.mod`, `internal/provider/dynamodb/client.go`.

**T-402 - Provider DynamoDB**
`GetItem` por `idcliente`, timeout proprio, conversao do item para `map[string]any` sem schema (RF-16).
Aceite: testes com cliente fake cobrindo item completo, item vazio, cliente inexistente, erro, timeout. [T-401]
Arquivos: `internal/provider/dynamodb/provider.go`.

**T-403 - Config do DynamoDB**
`eligibility.dynamodb.{table,partition_key,region,endpoint,timeout,max_retries}`, com `endpoint` para
DynamoDB Local.
Aceite: subir contra DynamoDB Local apontando o `endpoint`. [T-402]
Arquivos: `internal/config/config.go`, `configs/config.yaml`.

**T-404 - Teste de integracao**
Docker Compose com DynamoDB Local, seed de um cliente, teste marcado com build tag `integration`.
Aceite: `go test -tags=integration ./...` verde com o compose no ar. [T-403]

### Epico 5 - Origem HTTP

**T-501 - Config do bloco request**
`endpoint`, `method`, `query`, `headers`, `body` e o interpolador restrito da RF-21.
Aceite: variavel desconhecida gera erro de inicializacao citando a conversa.
Arquivos: `internal/provider/httpapi/request.go`.

**T-502 - Provider HTTP**
Cliente compartilhado com pool configurado, timeout efetivo `min(http.timeout, orcamento restante)`, decode do
JSON para `map[string]any`.
Aceite: testes com `httptest` para 200, 404, 500, corpo invalido, resposta lenta (timeout); teste que prova o
`min` do timeout quando o orcamento restante e menor. [T-501]
Arquivos: `internal/provider/httpapi/provider.go`.

**T-503 - Guardrails de custo, nao restricao de componente**
`source: http` liberado em qualquer componente. Na inicializacao: **aviso** quando o orcamento do componente e
menor que o timeout HTTP (RF-20), e **erro** quando o componente excede `max_calls` (RF-23).
Aceite: `source: http` em `banner.yaml` sobe a aplicacao e emite aviso nomeando componente e conversa; quatro
endpoints distintos no mesmo componente com `max_calls: 3` impedem o boot. [T-502, T-103]
Arquivos: `internal/provider/httpapi/validate.go`.

**T-504 - Circuit breaker**
Breaker por endpoint (janela deslizante, limiar de falha, tempo aberto), configuravel e desligavel. Breaker
aberto marca o candidato como `rejected_breaker_open` no trace.
Aceite: apos N falhas o breaker abre e as chamadas seguintes retornam imediatamente; fecha depois do
`open_duration`. [T-502]
Arquivos: `internal/provider/httpapi/breaker.go`.

**T-505 - Cache com TTL (opcional)**
Cache em memoria por endpoint + parametros, desligado por default.
Aceite: com TTL de 5s, duas requisicoes seguidas geram uma chamada. [T-502]
Arquivos: `internal/provider/httpapi/cache.go`.

### Epico 6 - Orcamento de tempo, paralelismo e fallback

**T-601 - Resolver de componente com orcamento**
`Resolve(ctx, component, client) Result` aplicando `min(timeout do componente, tempo restante do contexto)` e
traduzindo erro/timeout em `Reason`.
Aceite: componente com timeout de 50ms e provider de 200ms devolve fallback com `reason: timeout` em ~50ms.
[T-303]
Arquivos: `internal/selection/resolver.go`.

**T-602 - Execucao paralela dos slots**
Uma goroutine por slot; coleta por canal; deadline global no handler; slots pendentes viram fallback (RF-30).
Aceite: com um provider travado, a requisicao responde em ~1s com os slots rapidos preenchidos e o lento em
fallback; sem vazamento de goroutine (`goleak`). [T-601]
Arquivos: `internal/httpserver/handler.go`.

**T-603 - Politica de fallback**
Fallback do componente, escolhido por produto (RF-04), aplicado em `no_eligible`, `timeout`,
`provider_error`; `unknown_component` sem fallback (RF-33).
Aceite: uma tabela de testes cobre os quatro `reason` e a selecao por produto. [T-602, T-102]
Arquivos: `internal/selection/resolver.go`.

**T-604 - Novos DTOs**
`SelectionEntry{conversation, fallback, reason, adherence, elapsed_ms}`, `client.attributes` e `debug` no
request, `trace` opcional na resposta.
Aceite: contrato das RF-38 a RF-40 coberto por teste de serializacao; `debug: true` com
`expose_trace: false` nao vaza trace no corpo. [T-602, T-207]
Arquivos: `internal/httpserver/dto.go`.

**T-605 - Teste de carga**
Cenario com `footer` lento (600ms) e DynamoDB saudavel, medindo p99, com trace `off` e `on`.
Aceite: p99 <= 1s, footer p99 <= 500ms, zero 5xx, taxa de fallback consistente com a injecao de falha;
overhead do trace ligado documentado. [T-604]

### Epico 7 - Observabilidade e operacao

**T-701 - Middleware de request id e log de acesso**
`request_id` propagado no contexto e presente em todos os logs da requisicao.
Aceite: um log de acesso por requisicao com `elapsed_ms` e o resumo dos slots.
Arquivos: `internal/httpserver/middleware.go`.

**T-702 - Log da decisao do motor**
Emissao do trace conforme RF-34 a RF-37: resumo por slot sempre em `info`; trace completo em `debug` conforme
`log.decision_trace` (`off` / `sampled` / `on`) ou `debug: true` no request.
Aceite: com trace `on`, um unico log por slot permite responder "por que a conversa X nao apareceu" sem
reproduzir a requisicao; com trace `off`, nenhum campo de trace e emitido nem alocado. [T-701, T-207]
Arquivos: `internal/httpserver/middleware.go`, `internal/selection/resolver.go`, `internal/config/config.go`.

**T-703 - Metricas Prometheus**
Endpoint `/metrics` e os coletores da RNF-04, incluindo contador de rejeicao por `predicate_id` (cardinalidade
limitada ao inventario, conhecido na carga).
Aceite: metricas expostas com labels `component`, `source`, `reason`. [T-702]
Arquivos: `internal/metrics/metrics.go`.

**T-704 - Health e readiness**
`GET /health` e `GET /ready` (RF-41).
Aceite: `/ready` responde 503 enquanto o inventario nao estiver carregado.
Arquivos: `internal/httpserver/server.go`.

**T-705 - Shutdown gracioso**
`SIGTERM` para de aceitar conexoes e drena as requisicoes em voo respeitando o deadline global.
Aceite: requisicao em voo no momento do sinal termina com resposta valida.
Arquivos: `internal/app/app.go`.

### Epico 8 - Documentacao

**T-801 - Atualizar README**
Novo contrato de request/response, layout dos arquivos por componente, fallback por produto, exemplos.
Aceite: os curls do README funcionam contra a v1. [T-604]

**T-802 - Atualizar CLAUDE.md**
Nova arquitetura, procedimentos "adicionar componente" e "adicionar conversa" com regra de elegibilidade, e o
tradeoff de orcamento ao usar `source: http`.
Aceite: passo a passo reflete o codigo da v1. [T-801]

**T-803 - Guia de regras de elegibilidade**
`docs/eligibility.md`: operadores, sintaxe de caminho, calculo de aderencia, exemplos por origem e como ler o
trace de decisao no log.
Aceite: um analista consegue escrever uma regra e diagnosticar por que ela nao deu match sem ler codigo Go.
[T-208, T-702]

## 9. Fases de entrega

| Fase | Escopo | Entrega |
| --- | --- | --- |
| 1 | Epicos 0, 1 | Inventario por componente com fallback por produto, comportamento de selecao inalterado |
| 2 | Epicos 2, 3 | Motor de regras plugado com trace, ainda com `source: static` |
| 3 | Epico 4 | Elegibilidade real via DynamoDB |
| 4 | Epicos 5, 6 | Origem HTTP (usada no footer), orcamento de tempo e fallback garantido |
| 5 | Epicos 7, 8 | Observabilidade, log de decisao em producao, operacao e documentacao |

As fases 1 e 2 nao alteram o resultado observavel da API, o que permite validar a refatoracao antes de
introduzir I/O.

## 10. Riscos

| Risco | Impacto | Mitigacao |
| --- | --- | --- |
| API do footer lenta ou instavel | Fallback constante no footer | Breaker + cache opcional + alerta de taxa de fallback |
| Outro componente adotar `source: http` sem ajustar o orcamento | Timeout sistematico naquele slot | Aviso na inicializacao (RF-20), metrica de fallback por componente e `reason` |
| Regras com schema desconhecido silenciosamente sempre falsas | Inventario "morto" sem ninguem perceber | `fields_read` no trace, contador de rejeicao por `predicate_id` e alerta de conversa nunca selecionada |
| Trace ligado em producao com volume alto | Custo de log e latencia | Default `off`, modo `sampled`, truncamento de valores, coletor nulo sem alocacao |
| Valor sensivel da bag vazando para o log | Exposicao de dado de cliente | `log_value: false` por predicado, truncamento, revisao obrigatoria de regra nova |
| Custo e throttling do DynamoDB | Latencia e erro em pico | Uma leitura por requisicao, leitura eventual, alarme de throttle |
| Soma dos orcamentos maior que o deadline global | Requisicao estourando 1s | Validacao na inicializacao + orcamento por componente sempre limitado pelo tempo restante |
| Quebra de contrato da resposta | Integracao de consumidor | Sem consumidor em producao; alinhar antes da fase 4 |

## 11. Decisoes tomadas

1. **Conversa compartilhada entre componentes** - duplicar a entrada em cada arquivo. Nao ha deduplicacao: se
   a mesma conversa for elegivel em dois componentes, ela e exibida nos dois (RF-02).
2. **Recarga a quente do inventario** - fica para a v2. A v1 carrega na inicializacao.
3. **`min_adherence`** - removido. Elegibilidade e binaria; conversa que nao passa na regra e descartada e o
   motor segue para a proxima. Aderencia permanece apenas como desempate de ranking e sinal de diagnostico
   (RF-11, RF-12).
4. **Origem por conversa** - mantida. Sem default por arquivo na v1.
5. **Chamadas HTTP paralelas** - `max_calls` mantido, default `3`, agora configuravel por componente e nao
   exclusivo do footer (RF-23).
6. **Fallback** - um por produto, com os mesmos campos de uma conversa normal, mais um default obrigatorio
   para clientes sem contexto de produto (RF-03, RF-04).
7. **`source: http` fora do footer** - permitido. E um tradeoff de latencia sinalizado por aviso na
   inicializacao, nao uma restricao que impeca o boot (RF-20).

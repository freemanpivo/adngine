package httpserver_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/freemanpivo/adngine/internal/httpserver"
	"github.com/freemanpivo/adngine/internal/selection"
	"github.com/freemanpivo/adngine/internal/testsupport"
)

type selectionResponse struct {
	Selections map[string]*struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Product  string `json:"product"`
		Text     string `json:"text"`
		Link     string `json:"link"`
		Priority int    `json:"priority"`
	} `json:"selections"`
}

func newTestApp(t *testing.T) *fiber.App {
	t.Helper()

	return newAppWithRegistry(t, selection.NewRegistry(testsupport.LoadInventory(t), testsupport.Components))
}

func newAppWithRegistry(t *testing.T, registry *selection.Registry) *fiber.App {
	t.Helper()

	handler := httpserver.NewHandler(slog.New(slog.DiscardHandler), registry)

	app := fiber.New()
	app.Post("/v1/selections", handler.Select)
	return app
}

func post(t *testing.T, app *fiber.App, body string) (*http.Response, []byte) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/v1/selections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("executando requisicao: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("lendo corpo: %v", err)
	}
	return resp, raw
}

func decode(t *testing.T, raw []byte) selectionResponse {
	t.Helper()

	var out selectionResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decodificando resposta %s: %v", raw, err)
	}
	return out
}

func TestHandlerSelecionaUmaConversaPorSlot(t *testing.T) {
	app := newTestApp(t)

	resp, raw := post(t, app, `{
		"client": {"id": "cliente-1", "product": "veiculo"},
		"slots": ["banner", "card", "footer"]
	}`)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: esperava 200, recebeu %d (%s)", resp.StatusCode, raw)
	}

	got := decode(t, raw)
	want := map[string]string{
		"banner": "conv-veiculo-alta",
		"card":   "conv-veiculo-alta",
		"footer": "conv-empate-a",
	}
	if len(got.Selections) != len(want) {
		t.Fatalf("esperava %d slots, recebeu %d: %s", len(want), len(got.Selections), raw)
	}
	for slot, wantID := range want {
		entry, ok := got.Selections[slot]
		if !ok || entry == nil {
			t.Fatalf("slot %s ausente ou nulo: %s", slot, raw)
		}
		if entry.ID != wantID {
			t.Errorf("slot %s: esperava %s, recebeu %s", slot, wantID, entry.ID)
		}
	}
}

func TestHandlerExpoeOsCamposDaConversa(t *testing.T) {
	app := newTestApp(t)

	_, raw := post(t, app, `{"client": {"id": "cliente-1", "product": "veiculo"}, "slots": ["banner"]}`)

	entry := decode(t, raw).Selections["banner"]
	if entry == nil {
		t.Fatalf("slot banner nulo: %s", raw)
	}
	if entry.Type != "action" {
		t.Errorf("type: esperava action, recebeu %s", entry.Type)
	}
	if entry.Product != "veiculo" {
		t.Errorf("product: esperava veiculo, recebeu %s", entry.Product)
	}
	if entry.Priority != 10 {
		t.Errorf("priority: esperava 10, recebeu %d", entry.Priority)
	}
	if entry.Text == "" || entry.Link == "" {
		t.Errorf("text e link devem ser expostos: %s", raw)
	}
}

// Contrato atual: slot sem conversa elegivel e slot desconhecido sao ambos
// `null`, indistinguiveis para o consumidor. A RF-40 troca isso por uma entrada
// com fallback e reason.
func TestHandlerComponenteDesconhecidoRetornaNulo(t *testing.T) {
	app := newTestApp(t)

	resp, raw := post(t, app, `{
		"client": {"id": "cliente-1", "product": "veiculo"},
		"slots": ["banner", "sidebar"]
	}`)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: esperava 200, recebeu %d (%s)", resp.StatusCode, raw)
	}

	got := decode(t, raw)
	entry, ok := got.Selections["sidebar"]
	if !ok {
		t.Fatalf("slot desconhecido deve aparecer na resposta: %s", raw)
	}
	if entry != nil {
		t.Fatalf("slot desconhecido: esperava null, recebeu %+v", entry)
	}
	if got.Selections["banner"] == nil {
		t.Fatalf("slot desconhecido nao pode afetar os demais: %s", raw)
	}
}

// Produto sem inventario proprio ainda enxerga as conversas sem produto: a
// ausencia de match so acontece quando nao ha nenhuma conversa generica.
func TestHandlerProdutoDesconhecidoCaiNaConversaGenerica(t *testing.T) {
	app := newTestApp(t)

	_, raw := post(t, app, `{"client": {"id": "cliente-1", "product": "imovel"}, "slots": ["banner"]}`)

	entry := decode(t, raw).Selections["banner"]
	if entry == nil {
		t.Fatalf("slot banner nulo: %s", raw)
	}
	if entry.ID != "conv-generica" {
		t.Fatalf("esperava conv-generica, recebeu %s", entry.ID)
	}
}

func TestHandlerClienteSemProduto(t *testing.T) {
	app := newTestApp(t)

	_, raw := post(t, app, `{"client": {"id": "cliente-1"}, "slots": ["banner", "card"]}`)

	got := decode(t, raw)
	for _, slot := range []string{"banner", "card"} {
		entry := got.Selections[slot]
		if entry == nil {
			t.Fatalf("slot %s nulo: %s", slot, raw)
		}
		if entry.ID != "conv-generica" {
			t.Errorf("slot %s: esperava conv-generica, recebeu %s", slot, entry.ID)
		}
	}
}

func TestHandlerSlotRepetido(t *testing.T) {
	app := newTestApp(t)

	_, raw := post(t, app, `{"client": {"id": "cliente-1", "product": "veiculo"}, "slots": ["banner", "banner"]}`)

	got := decode(t, raw)
	if len(got.Selections) != 1 {
		t.Fatalf("slot repetido deve colapsar em uma entrada, recebeu %d: %s", len(got.Selections), raw)
	}
}

func TestHandlerRequisicaoInvalida(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		name string
		body string
	}{
		{name: "corpo malformado", body: `{"client": `},
		{name: "corpo vazio", body: ``},
		{name: "slots ausente", body: `{"client": {"id": "cliente-1"}}`},
		{name: "slots vazio", body: `{"client": {"id": "cliente-1"}, "slots": []}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, raw := post(t, app, tt.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status: esperava 400, recebeu %d (%s)", resp.StatusCode, raw)
			}

			var body map[string]string
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatalf("decodificando erro %s: %v", raw, err)
			}
			if body["error"] == "" {
				t.Errorf("esperava mensagem de erro, recebeu %s", raw)
			}
		})
	}
}

func TestHandlerInventarioVazio(t *testing.T) {
	repo := testsupport.LoadComponent(t, "banner", "empty/banner.yaml")
	app := newAppWithRegistry(t, selection.NewRegistry(repo, []string{"banner"}))

	resp, raw := post(t, app, `{"client": {"id": "cliente-1", "product": "veiculo"}, "slots": ["banner"]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: esperava 200, recebeu %d (%s)", resp.StatusCode, raw)
	}
	if entry := decode(t, raw).Selections["banner"]; entry != nil {
		t.Fatalf("esperava null com inventario vazio, recebeu %+v", entry)
	}
}

package config_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"os"

	"github.com/freemanpivo/adngine/internal/config"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("escrevendo config: %v", err)
	}
	return path
}

func TestLoadConfigCompleta(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, `
server:
  port: 9090
log:
  level: debug
selection:
  global_timeout: 2s
  components:
    banner:
      file_path: configs/conversations/banner.yaml
      timeout: 150ms
    footer:
      file_path: configs/conversations/footer.yaml
      timeout: 500ms
      max_calls: 5
`))
	if err != nil {
		t.Fatalf("esperava sucesso, recebeu %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("port: esperava 9090, recebeu %d", cfg.Server.Port)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("log.level: esperava debug, recebeu %s", cfg.Log.Level)
	}
	if cfg.Selection.GlobalTimeout != 2*time.Second {
		t.Errorf("global_timeout: esperava 2s, recebeu %s", cfg.Selection.GlobalTimeout)
	}
	if got := cfg.Selection.Components["banner"].Timeout; got != 150*time.Millisecond {
		t.Errorf("banner.timeout: esperava 150ms, recebeu %s", got)
	}
	if got := cfg.Selection.Components["footer"].MaxCalls; got != 5 {
		t.Errorf("footer.max_calls: esperava 5, recebeu %d", got)
	}
}

func TestLoadConfigSemBlocoSelectionUsaOsComponentesPadrao(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, "server:\n  port: 8080\n"))
	if err != nil {
		t.Fatalf("esperava sucesso, recebeu %v", err)
	}

	if got := cfg.Selection.GlobalTimeout; got != config.DefaultGlobalTimeout {
		t.Errorf("global_timeout: esperava %s, recebeu %s", config.DefaultGlobalTimeout, got)
	}

	want := map[string]time.Duration{
		"banner": config.DefaultComponentTimeout,
		"card":   config.DefaultComponentTimeout,
		"footer": 500 * time.Millisecond,
	}
	if len(cfg.Selection.Components) != len(want) {
		t.Fatalf("esperava %d componentes, recebeu %v", len(want), cfg.Selection.ComponentNames())
	}
	for name, timeout := range want {
		component, ok := cfg.Selection.Components[name]
		if !ok {
			t.Fatalf("componente %s ausente", name)
		}
		if component.Timeout != timeout {
			t.Errorf("%s.timeout: esperava %s, recebeu %s", name, timeout, component.Timeout)
		}
		if component.FilePath == "" {
			t.Errorf("%s.file_path: esperava default, recebeu vazio", name)
		}
		if component.MaxCalls != config.DefaultMaxCalls {
			t.Errorf("%s.max_calls: esperava %d, recebeu %d", name, config.DefaultMaxCalls, component.MaxCalls)
		}
	}
}

// Declarar componentes na config substitui a lista padrao inteira: remover um
// componente da config precisa de fato remove-lo.
func TestLoadConfigNaoInjetaComponentesPadraoQuandoHaDeclaracao(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, `
selection:
  components:
    banner:
      file_path: configs/conversations/banner.yaml
`))
	if err != nil {
		t.Fatalf("esperava sucesso, recebeu %v", err)
	}

	if got := cfg.Selection.ComponentNames(); len(got) != 1 || got[0] != "banner" {
		t.Fatalf("esperava apenas banner, recebeu %v", got)
	}
}

func TestLoadConfigCompletaTimeoutAusente(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, `
selection:
  components:
    footer:
      file_path: configs/conversations/footer.yaml
    sidebar:
      file_path: configs/conversations/sidebar.yaml
`))
	if err != nil {
		t.Fatalf("esperava sucesso, recebeu %v", err)
	}

	if got := cfg.Selection.Components["footer"].Timeout; got != 500*time.Millisecond {
		t.Errorf("footer sem timeout deve usar o default do componente, recebeu %s", got)
	}
	if got := cfg.Selection.Components["sidebar"].Timeout; got != config.DefaultComponentTimeout {
		t.Errorf("componente novo sem timeout deve usar o default geral, recebeu %s", got)
	}
}

func TestLoadConfigInvalida(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantIn  string
	}{
		{
			name: "timeout do componente maior que o global",
			content: `
selection:
  global_timeout: 300ms
  components:
    footer:
      file_path: configs/conversations/footer.yaml
      timeout: 500ms
`,
			wantIn: "maior que selection.global_timeout",
		},
		{
			name: "componente sem file_path",
			content: `
selection:
  components:
    banner:
      timeout: 200ms
`,
			wantIn: "file_path e obrigatorio",
		},
		{
			name:    "porta invalida",
			content: "server:\n  port: 70000\n",
			wantIn:  "server.port invalido",
		},
		{
			name:    "yaml malformado",
			content: "server:\n  port: \"sem aspas fechadas\n",
			wantIn:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.Load(writeConfig(t, tt.content))
			if err == nil {
				t.Fatal("esperava erro, recebeu nil")
			}
			if tt.wantIn != "" && !strings.Contains(err.Error(), tt.wantIn) {
				t.Fatalf("erro deveria conter %q, recebeu %q", tt.wantIn, err)
			}
		})
	}
}

func TestLoadConfigArquivoInexistente(t *testing.T) {
	if _, err := config.Load(filepath.Join(t.TempDir(), "nao_existe.yaml")); err == nil {
		t.Fatal("esperava erro, recebeu nil")
	}
}

func TestComponentNamesEOrdenado(t *testing.T) {
	cfg, err := config.Load(writeConfig(t, `
selection:
  components:
    footer:
      file_path: f.yaml
    banner:
      file_path: b.yaml
    card:
      file_path: c.yaml
`))
	if err != nil {
		t.Fatalf("esperava sucesso, recebeu %v", err)
	}

	got := cfg.Selection.ComponentNames()
	want := []string{"banner", "card", "footer"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("esperava %v, recebeu %v", want, got)
		}
	}
}

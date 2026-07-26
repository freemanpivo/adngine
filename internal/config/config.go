package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/spf13/viper"
)

const (
	DefaultGlobalTimeout    = time.Second
	DefaultComponentTimeout = 200 * time.Millisecond
	DefaultMaxCalls         = 3
)

// O footer nasce com orcamento maior porque e o componente que assume o custo de
// uma integracao externa.
var builtinComponents = map[string]ComponentConfig{
	"banner": {FilePath: "configs/conversations/banner.yaml", Timeout: DefaultComponentTimeout},
	"card":   {FilePath: "configs/conversations/card.yaml", Timeout: DefaultComponentTimeout},
	"footer": {FilePath: "configs/conversations/footer.yaml", Timeout: 500 * time.Millisecond},
}

type ComponentConfig struct {
	FilePath string        `mapstructure:"file_path"`
	Timeout  time.Duration `mapstructure:"timeout"`
	MaxCalls int           `mapstructure:"max_calls"`
}

type SelectionConfig struct {
	GlobalTimeout time.Duration              `mapstructure:"global_timeout"`
	Components    map[string]ComponentConfig `mapstructure:"components"`
}

// ComponentNames devolve os componentes em ordem estavel, para que carga,
// validacao e log nao dependam da iteracao aleatoria do mapa.
func (s SelectionConfig) ComponentNames() []string {
	return slices.Sorted(maps.Keys(s.Components))
}

type Config struct {
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`

	Log struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"log"`

	Selection SelectionConfig `mapstructure:"selection"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	v.SetDefault("server.port", 8080)
	v.SetDefault("log.level", "info")
	v.SetDefault("selection.global_timeout", DefaultGlobalTimeout)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("lendo arquivo de config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parseando arquivo de config: %w", err)
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config invalida (%s): %w", path, err)
	}

	return &cfg, nil
}

// Os defaults por componente so entram quando a config nao declara componente
// algum. Assim, remover um componente da config realmente o remove.
func (c *Config) applyDefaults() {
	if c.Selection.GlobalTimeout <= 0 {
		c.Selection.GlobalTimeout = DefaultGlobalTimeout
	}

	if len(c.Selection.Components) == 0 {
		c.Selection.Components = maps.Clone(builtinComponents)
	}

	for name, component := range c.Selection.Components {
		if component.Timeout <= 0 {
			component.Timeout = DefaultComponentTimeout
			if builtin, ok := builtinComponents[name]; ok {
				component.Timeout = builtin.Timeout
			}
		}
		if component.MaxCalls <= 0 {
			component.MaxCalls = DefaultMaxCalls
		}
		c.Selection.Components[name] = component
	}
}

func (c *Config) validate() error {
	var errs []error

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("server.port invalido: %d", c.Server.Port))
	}
	if len(c.Selection.Components) == 0 {
		errs = append(errs, errors.New("selection.components: pelo menos um componente e obrigatorio"))
	}

	for _, name := range c.Selection.ComponentNames() {
		component := c.Selection.Components[name]

		if component.FilePath == "" {
			errs = append(errs, fmt.Errorf("selection.components.%s.file_path e obrigatorio", name))
		}
		if component.Timeout > c.Selection.GlobalTimeout {
			errs = append(errs, fmt.Errorf(
				"selection.components.%s.timeout (%s) maior que selection.global_timeout (%s)",
				name, component.Timeout, c.Selection.GlobalTimeout))
		}
	}

	return errors.Join(errs...)
}

package llm

import (
	"errors"
	"time"
)

const (
	defaultTimeout     = 30 * time.Second
	defaultMaxAttempts = 3
	defaultBaseBackoff = 500 * time.Millisecond
	maxBackoff         = 8 * time.Second
)

type Config struct {
	Provider    string
	Model       string
	Endpoint    string
	APIKey      string
	Timeout     time.Duration
	MaxAttempts int
	BaseBackoff time.Duration
}

// Validate leaves the endpoint out: an empty one falls back to the provider's
// own url, so only the model and the key have to be supplied.
func (c Config) Validate() error {
	if c.Model == "" || c.APIKey == "" {
		return errors.New("llm config requires a model and an api key")
	}
	return nil
}

func (c Config) withDefaults() Config {
	if c.Provider == "" {
		c.Provider = ProviderAnthropic
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultMaxAttempts
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = defaultBaseBackoff
	}
	return c
}

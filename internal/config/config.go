package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2"
)

type Config struct {
	HTTPPort string
	Google   GoogleOAuthConfig
}

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

func Load() (*Config, error) {
	config := &Config{
		HTTPPort: valueOrDefault("HTTP_PORT", "8080"),
		Google: GoogleOAuthConfig{
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
			Scopes:       parseScopes(os.Getenv("GOOGLE_SCOPES")),
		},
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if strings.TrimSpace(c.Google.ClientID) == "" {
		return errors.New("GOOGLE_CLIENT_ID is required")
	}
	if strings.TrimSpace(c.Google.ClientSecret) == "" {
		return errors.New("GOOGLE_CLIENT_SECRET is required")
	}
	if strings.TrimSpace(c.Google.RedirectURL) == "" {
		return errors.New("GOOGLE_REDIRECT_URL is required")
	}
	if len(c.Google.Scopes) == 0 {
		return errors.New("GOOGLE_SCOPES is required")
	}
	return nil
}

func (c *Config) GoogleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.Google.ClientID,
		ClientSecret: c.Google.ClientSecret,
		RedirectURL:  c.Google.RedirectURL,
		Scopes:       c.Google.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}
}

func valueOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func parseScopes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{"openid", "email"}
	}

	parts := strings.Split(raw, ",")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		if scope := strings.TrimSpace(part); scope != "" {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

func (c Config) String() string {
	return fmt.Sprintf("http_port=%s google_scopes=%s", c.HTTPPort, strings.Join(c.Google.Scopes, ","))
}

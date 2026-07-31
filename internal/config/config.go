package config

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type Config struct {
	HTTPPort      string
	Google        GoogleOAuthConfig
	Database      DatabaseConfig
	EncryptionKey []byte
}

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

type DatabaseConfig struct {
	URL string // PostgreSQL connection string
	DB  *gorm.DB
}

func Load() (*Config, error) {
	// Load and validate encryption key (must be 32 bytes for AES-256)
	encryptionKeyStr := valueOrDefault("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	var encryptionKey []byte
	if encryptionKeyStr != "" {
		// Key should be a hex-encoded 32-byte string
		decoded, err := decodeHexKey(encryptionKeyStr)
		if err != nil {
			return nil, fmt.Errorf("invalid ENCRYPTION_KEY: %w", err)
		}
		encryptionKey = decoded
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")

	if clientID == "" || clientSecret == "" || redirectURL == "" {
		fClientID, fClientSecret, fRedirectURL, err := loadGoogleCredentialsFile()
		if err == nil {
			if clientID == "" {
				clientID = fClientID
			}
			if clientSecret == "" {
				clientSecret = fClientSecret
			}
			if redirectURL == "" {
				redirectURL = fRedirectURL
			}
		}
	}

	config := &Config{
		HTTPPort: valueOrDefault("HTTP_PORT", "8080"),
		Google: GoogleOAuthConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       parseScopes(os.Getenv("GOOGLE_SCOPES")),
		},
		Database: DatabaseConfig{
			URL: valueOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cumulus?sslmode=disable"),
		},
		EncryptionKey: encryptionKey,
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
	if strings.TrimSpace(c.Database.URL) == "" {
		return errors.New("DATABASE_URL is required")
	}
	if len(c.EncryptionKey) == 0 {
		return errors.New("ENCRYPTION_KEY is required (must be 32 bytes hex-encoded)")
	}
	if len(c.EncryptionKey) != 32 {
		return errors.New("ENCRYPTION_KEY must be exactly 32 bytes")
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
		return []string{"openid", "email", "https://www.googleapis.com/auth/drive"}
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

type googleCredentialsFile struct {
	Web *struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		RedirectURIs []string `json:"redirect_uris"`
	} `json:"web"`
	Installed *struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		RedirectURIs []string `json:"redirect_uris"`
	} `json:"installed"`
}

func loadGoogleCredentialsFile() (clientID, clientSecret, redirectURL string, err error) {
	candidates := []string{
		"credentials/client_secret.json",
		"../credentials/client_secret.json",
		"../../credentials/client_secret.json",
	}
	if matches, _ := filepath.Glob("credentials/client_secret*.json"); len(matches) > 0 {
		candidates = append(candidates, matches...)
	}
	if matches, _ := filepath.Glob("../credentials/client_secret*.json"); len(matches) > 0 {
		candidates = append(candidates, matches...)
	}
	if matches, _ := filepath.Glob("../../credentials/client_secret*.json"); len(matches) > 0 {
		candidates = append(candidates, matches...)
	}
	if matches, _ := filepath.Glob("credentials/*.json"); len(matches) > 0 {
		candidates = append(candidates, matches...)
	}

	var filePath string
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			filePath = path
			break
		}
	}

	if filePath == "" {
		return "", "", "", errors.New("credentials file not found in credentials/")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds googleCredentialsFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", "", "", fmt.Errorf("failed to unmarshal credentials file: %w", err)
	}

	if creds.Web != nil {
		clientID = creds.Web.ClientID
		clientSecret = creds.Web.ClientSecret
		if len(creds.Web.RedirectURIs) > 0 {
			redirectURL = creds.Web.RedirectURIs[0]
		}
	} else if creds.Installed != nil {
		clientID = creds.Installed.ClientID
		clientSecret = creds.Installed.ClientSecret
		if len(creds.Installed.RedirectURIs) > 0 {
			redirectURL = creds.Installed.RedirectURIs[0]
		}
	}

	return clientID, clientSecret, redirectURL, nil
}

func (c Config) String() string {
	return fmt.Sprintf("http_port=%s google_scopes=%s database_url=%s", c.HTTPPort, strings.Join(c.Google.Scopes, ","), c.Database.URL)
}

func decodeHexKey(hexKey string) ([]byte, error) {
	decoded, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("key must decode to 32 bytes, got %d bytes", len(decoded))
	}
	return decoded, nil
}

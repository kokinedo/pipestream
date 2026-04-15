package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Credentials stores API keys for multiple providers.
type Credentials struct {
	DefaultProvider string                    `json:"default_provider"`
	Providers       map[string]ProviderCreds  `json:"providers"`
}

// ProviderCreds holds a single provider's credentials.
type ProviderCreds struct {
	APIKey string `json:"api_key"`
}

var envVars = map[string]string{
	"claude": "ANTHROPIC_API_KEY",
	"openai": "OPENAI_API_KEY",
	"gemini": "GEMINI_API_KEY",
}

// CredentialsPath returns the path to the credentials file.
func CredentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pipestream", "credentials.json")
}

// Load reads credentials from disk.
func Load() *Credentials {
	path := CredentialsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return &Credentials{DefaultProvider: "claude", Providers: map[string]ProviderCreds{}}
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return &Credentials{DefaultProvider: "claude", Providers: map[string]ProviderCreds{}}
	}
	if creds.Providers == nil {
		creds.Providers = map[string]ProviderCreds{}
	}
	return &creds
}

// Save writes credentials to disk.
func Save(creds *Credentials) error {
	path := CredentialsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// GetAPIKey returns the API key for a provider (credentials file, then env var).
func GetAPIKey(provider string) string {
	creds := Load()
	if pc, ok := creds.Providers[provider]; ok && pc.APIKey != "" {
		return pc.APIKey
	}
	if envVar, ok := envVars[provider]; ok {
		return os.Getenv(envVar)
	}
	return ""
}

// GetDefaultProvider returns the default provider name.
func GetDefaultProvider() string {
	creds := Load()
	if creds.DefaultProvider != "" {
		return creds.DefaultProvider
	}
	return "claude"
}

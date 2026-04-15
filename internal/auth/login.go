package auth

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var consoleURLs = map[string]string{
	"claude": "https://console.anthropic.com/settings/keys",
	"openai": "https://platform.openai.com/api-keys",
	"gemini": "https://aistudio.google.com/apikey",
}

var providerNames = map[string]string{
	"claude": "Anthropic (Claude)",
	"openai": "OpenAI",
	"gemini": "Google (Gemini)",
}

// Login runs an interactive login flow for a provider.
func Login(provider string) error {
	name := providerNames[provider]
	if name == "" {
		name = provider
	}

	fmt.Printf("\n  Logging in to %s...\n\n", name)

	if url, ok := consoleURLs[provider]; ok {
		fmt.Printf("  Opening %s\n\n", url)
		openBrowser(url)
	}

	fmt.Print("  Paste your API key: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	apiKey := strings.TrimSpace(scanner.Text())

	if apiKey == "" {
		fmt.Println("  No key provided. Aborting.")
		return nil
	}

	creds := Load()
	creds.Providers[provider] = ProviderCreds{APIKey: apiKey}
	creds.DefaultProvider = provider
	if err := Save(creds); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	fmt.Printf("\n  Logged in to %s successfully.\n", name)
	fmt.Printf("  Credentials saved to %s\n\n", CredentialsPath())
	return nil
}

// Logout removes stored credentials for a provider.
func Logout(provider string) error {
	creds := Load()
	if _, ok := creds.Providers[provider]; ok {
		delete(creds.Providers, provider)
		if err := Save(creds); err != nil {
			return err
		}
		name := providerNames[provider]
		if name == "" {
			name = provider
		}
		fmt.Printf("\n  Logged out of %s.\n\n", name)
	} else {
		fmt.Printf("\n  No stored credentials for %s.\n\n", provider)
	}
	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		cmd = exec.Command("open", url)
	}
	_ = cmd.Start()
}

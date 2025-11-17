package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileProvider retrieves secrets from files (Kubernetes secret mounts)
// Follows the Kubernetes secret mount pattern where each secret is a file
// Example: /var/secrets/claude-api-key, /var/secrets/jwt-secret
type FileProvider struct {
	secretsPath string
}

// NewFileProvider creates a new file-based secret provider
// secretsPath is the directory where secret files are mounted (e.g., "/var/secrets")
func NewFileProvider(secretsPath string) *FileProvider {
	return &FileProvider{
		secretsPath: secretsPath,
	}
}

// GetSecret retrieves a secret from a file
// The key is converted to a filename by replacing underscores with hyphens and lowercasing
// Example: CLAUDE_API_KEY -> claude-api-key
func (f *FileProvider) GetSecret(ctx context.Context, key string) (string, error) {
	if f.secretsPath == "" {
		return "", fmt.Errorf("secrets path not configured")
	}

	// Convert env var name to kubernetes-style secret file name
	// CLAUDE_API_KEY -> claude-api-key
	filename := strings.ToLower(strings.ReplaceAll(key, "_", "-"))
	filepath := filepath.Join(f.secretsPath, filename)

	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// Not found is not an error, just return empty string
			return "", nil
		}
		return "", fmt.Errorf("failed to read secret file %s: %w", filepath, err)
	}

	// Trim whitespace/newlines that might be in the file
	return strings.TrimSpace(string(data)), nil
}

// Name returns the provider name
func (f *FileProvider) Name() string {
	return "file"
}

// IsAvailable checks if the secrets directory exists
func (f *FileProvider) IsAvailable(ctx context.Context) bool {
	if f.secretsPath == "" {
		return false
	}

	info, err := os.Stat(f.secretsPath)
	if err != nil {
		return false
	}

	return info.IsDir()
}

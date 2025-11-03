// internal/mimir/client.go
package mimir

import "time"

// AuthConfig holds authentication configuration for Mimir
type AuthConfig struct {
	Username string
	Password string
	Token    string
}

type Client struct {
	endpoint string
	auth     AuthConfig
	timeout  time.Duration
}
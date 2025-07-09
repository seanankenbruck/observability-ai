// internal/mimir/client.go
type Client struct {
	endpoint string
	auth     AuthConfig
	timeout  time.Duration
}
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ancalabrese/ws1cli/ws1"
	"github.com/spf13/cobra"
)

var (
	server string
	token  string
	tenant string
)

var RootCmd = &cobra.Command{
	Use:          "ws1",
	Short:        "Command-line interface for Workspace ONE UEM.",
	SilenceUsage: true,
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&server, "server", os.Getenv("WS1_SERVER"), "Server hostname override [WS1_SERVER]")
	RootCmd.PersistentFlags().StringVar(&token, "token", os.Getenv("WS1_TOKEN"), "Bearer token override [WS1_TOKEN]")
	RootCmd.PersistentFlags().StringVar(&tenant, "tenant", os.Getenv("WS1_TENANT"), "aw-tenant-code override [WS1_TENANT]")
}

// effectiveConfig merges the saved config file with any CLI flag overrides.
// It is the single source of truth for resolved configuration.
func effectiveConfig() config {
	cfg, _ := loadConfig()
	if server != "" {
		cfg.Server = server
	}
	if tenant != "" {
		cfg.Tenant = tenant
	}
	if token != "" {
		// Explicit token override: use it as-is and disable auto-refresh,
		// since we don't know its expiry and didn't issue it ourselves.
		cfg.Token = token
		cfg.ExpiresAt = 0
		cfg.ClientID = ""
		cfg.ClientSecret = ""
		cfg.TokenEndpoint = ""
	}
	return cfg
}

type authTransport struct {
	mu            sync.Mutex
	token         string
	tenant        string
	clientID      string
	clientSecret  string
	tokenEndpoint string
	expiresAt     int64 // unix timestamp; 0 means no expiry tracking
	base          http.RoundTripper
}

func (t *authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.mu.Lock()
	if t.clientID != "" && t.clientSecret != "" && t.expiresAt > 0 &&
		time.Now().Unix() >= t.expiresAt-60 {
		if resp, err := fetchOAuthToken(t.tokenEndpoint, t.clientID, t.clientSecret); err == nil {
			t.token = resp.AccessToken
			t.expiresAt = time.Now().Unix() + int64(resp.ExpiresIn)
			if cfg, err := loadConfig(); err == nil {
				cfg.Token = t.token
				cfg.ExpiresAt = t.expiresAt
				_ = saveConfig(cfg)
			}
		}
	}
	tok := t.token
	ten := t.tenant
	t.mu.Unlock()

	r = r.Clone(r.Context())
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	if ten != "" {
		r.Header.Set("aw-tenant-code", ten)
	}
	r.Header.Set("Accept", "application/json")
	return t.base.RoundTrip(r)
}

func NewClient() (*ws1.ClientWithResponses, error) {
	cfg := effectiveConfig()

	if cfg.Server == "" {
		return nil, errors.New("server not configured — run 'ws1 config' first")
	}
	if cfg.Token == "" {
		return nil, errors.New("not authenticated — run 'ws1 login'")
	}

	base := &http.Client{
		Transport: &authTransport{
			token:         cfg.Token,
			tenant:        cfg.Tenant,
			clientID:      cfg.ClientID,
			clientSecret:  cfg.ClientSecret,
			tokenEndpoint: cfg.TokenEndpoint,
			expiresAt:     cfg.ExpiresAt,
			base:          http.DefaultTransport,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return ws1.NewClientWithResponses("https://"+cfg.Server+"/API", ws1.WithHTTPClient(base))
}

func PrintJSON(statusCode int, body []byte) error {
	if statusCode != 0 && (statusCode < 200 || statusCode >= 300) {
		return fmt.Errorf("HTTP %d: %s", statusCode, strings.TrimSpace(string(body)))
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		_, _ = os.Stdout.Write(body)
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

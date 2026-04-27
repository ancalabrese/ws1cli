package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

var loginFlags struct {
	clientID string
	secret   string
	region   string
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Workspace ONE UEM.",
	RunE:  runLogin,
}

func init() {
	RootCmd.AddCommand(loginCmd)
	loginCmd.Flags().StringVar(&loginFlags.clientID, "client-id", "", "OAuth2 client ID (required)")
	loginCmd.Flags().StringVar(&loginFlags.secret, "secret", "", "OAuth2 client secret (required)")
	loginCmd.Flags().StringVar(&loginFlags.region, "region", "na", "Omnissa auth region: na, emea, apac, uat")
	_ = loginCmd.MarkFlagRequired("client-id")
	_ = loginCmd.MarkFlagRequired("secret")
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func runLogin(cmd *cobra.Command, args []string) error {
	cfg := effectiveConfig()
	if cfg.Server == "" {
		return errors.New("server not configured — run 'ws1 config --server <hostname>' first")
	}

	out := cmd.OutOrStdout()
	tokenEndpoint := fmt.Sprintf("https://%s.uemauth.vmwservices.com/connect/token", loginFlags.region)
	fmt.Fprintf(out, "Authenticating with %s...\n", tokenEndpoint)

	oauthResp, err := fetchOAuthToken(tokenEndpoint, loginFlags.clientID, loginFlags.secret)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	cfg.Token = oauthResp.AccessToken
	cfg.ClientID = loginFlags.clientID
	cfg.ClientSecret = loginFlags.secret
	cfg.TokenEndpoint = tokenEndpoint
	cfg.ExpiresAt = time.Now().Unix() + int64(oauthResp.ExpiresIn)
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	consoleURL := "https://" + cfg.Server
	fmt.Fprintf(out, "\nOpen your WS1 console: %s\n\n", termLink(consoleURL, consoleURL))
	fmt.Fprintf(out, "You're all set!")
	if info := jwtDisplayName(oauthResp.AccessToken); info != "" {
		fmt.Fprintf(out, " Logged in as %s.", info)
	}
	fmt.Fprintln(out)
	return nil
}

func fetchOAuthToken(endpoint, clientID, secret string) (*oauthTokenResponse, error) {
	resp, err := http.PostForm(endpoint, url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {secret},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tr oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, fmt.Errorf("decoding response (HTTP %d): %w", resp.StatusCode, err)
	}
	if tr.Error != "" {
		return nil, fmt.Errorf("%s: %s", tr.Error, tr.Description)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in response (HTTP %d)", resp.StatusCode)
	}
	return &tr, nil
}

// jwtDisplayName extracts a human-readable identifier from a JWT's payload claims.
// Returns empty string if the token isn't a valid JWT or has no useful claims.
func jwtDisplayName(jwtToken string) string {
	parts := splitN(jwtToken, ".", 3)
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Name              string `json:"name"`
		Email             string `json:"email"`
		PreferredUsername string `json:"preferred_username"`
		Sub               string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	switch {
	case claims.Name != "":
		return claims.Name
	case claims.PreferredUsername != "":
		return claims.PreferredUsername
	case claims.Email != "":
		return claims.Email
	case claims.Sub != "":
		return claims.Sub
	}
	return ""
}

// termLink returns an OSC 8 hyperlink escape sequence for terminals that support it.
func termLink(text, href string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", href, text)
}

// splitN splits s by sep into at most n parts without importing strings.
func splitN(s, sep string, n int) []string {
	var parts []string
	for len(parts) < n-1 {
		i := indexOf(s, sep)
		if i < 0 {
			break
		}
		parts = append(parts, s[:i])
		s = s[i+len(sep):]
	}
	return append(parts, s)
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

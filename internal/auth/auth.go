// Package auth handles Google OAuth2 login so ytui can call the YouTube
// Data API on the user's behalf (real subscriptions, playlists, liked
// videos) instead of only the locally-tracked lists.
//
// This uses the standard "installed app" / desktop loopback flow: we spin
// up a local HTTP server on 127.0.0.1, open the user's browser to Google's
// consent page, and catch the redirect back to our local server once they
// approve. Nothing ever touches a public server — the whole exchange stays
// on localhost.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Scope is intentionally minimal — read-only access to subscriptions,
// playlists, and liked videos. Never requests write/manage access.
const Scope = "https://www.googleapis.com/auth/youtube.readonly"

// clientSecretsFile is exactly the format Google's Cloud Console lets you
// download directly when you create a Desktop-app OAuth client — so setup
// is "download the file, drop it here", not manually copying fields.
type clientSecretsFile struct {
	Installed struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		AuthURI      string `json:"auth_uri"`
		TokenURI     string `json:"token_uri"`
	} `json:"installed"`
}

func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "ytui")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func clientSecretsPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "client_secret.json"), nil
}

func tokenPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "token.json"), nil
}

// HaveClientSecrets reports whether the user has completed the one-time
// Google Cloud Console setup and dropped their client_secret.json in place.
func HaveClientSecrets() bool {
	path, err := clientSecretsPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// HaveToken reports whether a saved login already exists.
func HaveToken() bool {
	path, err := tokenPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// ClientSecretsPath is exposed so the UI can tell the user exactly where
// to put the file they download from Google Cloud Console.
func ClientSecretsPath() string {
	p, _ := clientSecretsPath()
	return p
}

// ImportClientCredentials builds the client_secret.json from a Client ID
// and Client Secret pasted directly from Google Cloud Console's
// confirmation dialog — no file download needed. The AuthURI/TokenURI
// fields in Google's JSON format aren't actually used anywhere in this
// package (loadOAuthConfig hardcodes google.Endpoint), so this minimal
// version is functionally identical to a downloaded file.
func ImportClientCredentials(clientID, clientSecret string) error {
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	if clientID == "" {
		return fmt.Errorf("client ID is empty")
	}
	if clientSecret == "" {
		return fmt.Errorf("client secret is empty")
	}

	var secrets clientSecretsFile
	secrets.Installed.ClientID = clientID
	secrets.Installed.ClientSecret = clientSecret

	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return err
	}
	dstPath, err := clientSecretsPath()
	if err != nil {
		return err
	}
	tmp := dstPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, dstPath)
}

// ImportClientSecretsRaw validates raw JSON text (e.g. pasted directly
// from the downloaded file) and saves it into place — the paste-friendly
// sibling of ImportClientSecrets, which takes a file path instead.
func ImportClientSecretsRaw(jsonText string) error {
	data := []byte(jsonText)
	var secrets clientSecretsFile
	if err := json.Unmarshal(data, &secrets); err != nil {
		return fmt.Errorf("that doesn't look like valid JSON — did you copy the whole file?")
	}
	if secrets.Installed.ClientID == "" {
		return fmt.Errorf("missing client_id — make sure you created a Desktop app credential, not Web app")
	}

	dstPath, err := clientSecretsPath()
	if err != nil {
		return err
	}
	tmp := dstPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, dstPath)
}

// ImportClientSecrets validates the file at srcPath looks like a Google
// Cloud Console "Desktop app" OAuth client download, then copies it into
// place at the standard location — so the person never has to manually
// mv/rename anything themselves.
func ImportClientSecrets(srcPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("couldn't read %s: %w", srcPath, err)
	}
	var secrets clientSecretsFile
	if err := json.Unmarshal(data, &secrets); err != nil {
		return fmt.Errorf("that doesn't look like valid JSON — did you download the right file?")
	}
	if secrets.Installed.ClientID == "" {
		return fmt.Errorf("missing client_id — make sure you created a Desktop app credential, not Web app")
	}

	dstPath, err := clientSecretsPath()
	if err != nil {
		return err
	}
	tmp := dstPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, dstPath)
}

func loadOAuthConfig() (*oauth2.Config, error) {
	path, err := clientSecretsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no client_secret.json found — see the README for the Google Cloud Console setup steps")
	}
	var secrets clientSecretsFile
	if err := json.Unmarshal(data, &secrets); err != nil {
		return nil, fmt.Errorf("client_secret.json is malformed: %w", err)
	}
	if secrets.Installed.ClientID == "" {
		return nil, fmt.Errorf("client_secret.json is missing client_id — make sure you downloaded a Desktop app credential, not a Web app one")
	}
	return &oauth2.Config{
		ClientID:     secrets.Installed.ClientID,
		ClientSecret: secrets.Installed.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{Scope},
		// RedirectURL is set per-attempt once we know which local port we
		// actually got — see Login below.
	}, nil
}

func saveToken(tok *oauth2.Token) error {
	path, err := tokenPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadToken() (*oauth2.Token, error) {
	path, err := tokenPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

// Logout deletes the saved token. The client_secret.json (your app
// registration) is left alone — that's not a login, it's the app itself.
func Logout() error {
	path, err := tokenPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("don't know how to open a browser on %s", runtime.GOOS)
	}
	return cmd.Start()
}

// Login runs the full interactive OAuth flow: starts a local callback
// server, opens the browser to Google's consent screen, waits for the
// redirect, exchanges the code for a token, and saves it. Blocks until the
// user completes (or abandons) the flow in their browser, so callers
// should run this off the UI goroutine (it's designed to be wrapped in a
// Bubbletea tea.Cmd).
func Login(ctx context.Context) error {
	cfg, err := loadOAuthConfig()
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("couldn't start local callback server: %w", err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	cfg.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	state, err := randomState()
	if err != nil {
		return err
	}

	type result struct {
		token *oauth2.Token
		err   error
	}
	resultCh := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "state mismatch — possible CSRF, aborting", http.StatusBadRequest)
			resultCh <- result{err: fmt.Errorf("OAuth state mismatch")}
			return
		}
		if errMsg := q.Get("error"); errMsg != "" {
			fmt.Fprintf(w, "<html><body>Login cancelled (%s). You can close this window.</body></html>", errMsg)
			resultCh <- result{err: fmt.Errorf("login cancelled: %s", errMsg)}
			return
		}
		code := q.Get("code")
		tok, err := cfg.Exchange(ctx, code)
		if err != nil {
			http.Error(w, "token exchange failed", http.StatusInternalServerError)
			resultCh <- result{err: err}
			return
		}
		fmt.Fprint(w, "<html><body><h2>ytui: logged in ✓</h2>You can close this window and go back to the terminal.</body></html>")
		resultCh <- result{token: tok}
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	if err := openBrowser(authURL); err != nil {
		return fmt.Errorf("couldn't open your browser automatically — open this URL manually: %s", authURL)
	}

	select {
	case res := <-resultCh:
		if res.err != nil {
			return res.err
		}
		return saveToken(res.token)
	case <-time.After(3 * time.Minute):
		return fmt.Errorf("login timed out waiting for browser approval")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// persistingTokenSource saves the token to disk whenever oauth2 silently
// refreshes it, so the next launch doesn't have to log in again just
// because the short-lived access token expired.
type persistingTokenSource struct {
	base oauth2.TokenSource
	last string
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	if tok.AccessToken != p.last {
		_ = saveToken(tok) // best-effort; a failed persist just means one extra re-auth later
		p.last = tok.AccessToken
	}
	return tok, nil
}

// Client returns an *http.Client authenticated with the saved login,
// automatically refreshing (and re-persisting) the access token as needed.
// Returns an error if there's no saved login.
func Client(ctx context.Context) (*http.Client, error) {
	cfg, err := loadOAuthConfig()
	if err != nil {
		return nil, err
	}
	tok, err := loadToken()
	if err != nil {
		return nil, fmt.Errorf("not logged in")
	}
	src := &persistingTokenSource{base: cfg.TokenSource(ctx, tok), last: tok.AccessToken}
	return oauth2.NewClient(ctx, src), nil
}

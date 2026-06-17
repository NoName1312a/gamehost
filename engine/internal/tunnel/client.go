// Package tunnel gives a hosted game server a shareable public address
// (<slug>.gn.coderaum.com:<port>) without port-forwarding, by talking to the
// GameNest control-plane and running a bundled frpc sidecar. It mirrors the
// playit relay feature but uses self-hosted infrastructure: the engine holds one
// anonymous device token, allocates public ports per server from the
// control-plane, and supervises a single frpc child process.
//
// The feature is dormant unless the engine is started with a control-plane URL
// (GAMEHOST_TUNNEL_URL); see cmd/engine.
package tunnel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/leop1/gamehost/engine/internal/secret"
)

// secretPrefix marks the new (DPAPI-encrypted, base64) on-disk token format,
// distinguishing it from any legacy plaintext token so both load correctly.
// Identical to the relay package's convention.
const secretPrefix = "gh1:"

// PortReq is one public port requested from the control-plane.
type PortReq struct {
	Role  string `json:"role"`
	Proto string `json:"proto"`
}

// AllocProxy is one allocated public port returned by the control-plane.
type AllocProxy struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	Proto      string `json:"proto"`
	RemotePort int    `json:"remotePort"`
	Address    string `json:"address"`
}

// Allocation is the control-plane's response for one slug: the frps coordinates
// to dial, a per-allocation secret to bind each proxy to this device, and the
// allocated public proxies.
type Allocation struct {
	Slug      string
	FrpsAddr  string
	FrpsToken string
	Secret    string
	Proxies   []AllocProxy
}

// Client is the control-plane HTTP client plus the device's bearer token.
type Client struct {
	baseURL string
	hc      *http.Client
	dataDir string

	mu    sync.Mutex // guards register-once + token read/write
	token string     // in-memory cache of the device token
}

// NewClient returns a control-plane client storing its device token under
// dataDir. baseURL is the control-plane root (e.g. https://cp.coderaum.com).
func NewClient(baseURL, dataDir string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		hc:      &http.Client{Timeout: 15 * time.Second},
		dataDir: dataDir,
	}
}

func (c *Client) tokenFile() string { return filepath.Join(c.dataDir, "tunnel-token") }

// loadToken reads the device token from disk, decrypting the DPAPI format and
// falling back to legacy plaintext. Empty string if absent/unreadable.
func (c *Client) loadToken() string {
	b, err := os.ReadFile(c.tokenFile())
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(b))
	rest, ok := strings.CutPrefix(content, secretPrefix)
	if !ok {
		return content // legacy plaintext
	}
	enc, err := base64.StdEncoding.DecodeString(rest)
	if err != nil {
		return ""
	}
	dec, err := secret.Unprotect(enc)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(dec))
}

// storeToken writes the device token DPAPI-encrypted (gh1: + base64).
func (c *Client) storeToken(token string) error {
	if err := os.MkdirAll(c.dataDir, 0o755); err != nil {
		return err
	}
	enc, err := secret.Protect([]byte(token))
	if err != nil {
		return err
	}
	data := secretPrefix + base64.StdEncoding.EncodeToString(enc)
	return os.WriteFile(c.tokenFile(), []byte(data), 0o600)
}

// ensureDevice returns the device bearer token, registering a new anonymous
// device with the control-plane (and persisting the token) on first use.
func (c *Client) ensureDevice(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" {
		return c.token, nil
	}
	if t := c.loadToken(); t != "" {
		c.token = t
		return t, nil
	}
	var resp struct {
		DeviceID string `json:"deviceId"`
		Token    string `json:"token"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/register", "", nil, &resp); err != nil {
		return "", err
	}
	if resp.Token == "" {
		return "", fmt.Errorf("control-plane register returned no token")
	}
	if err := c.storeToken(resp.Token); err != nil {
		return "", fmt.Errorf("persist device token: %w", err)
	}
	c.token = resp.Token
	return resp.Token, nil
}

// Allocate reserves public ports for slug and returns the allocation.
func (c *Client) Allocate(ctx context.Context, slug string, ports []PortReq) (Allocation, error) {
	token, err := c.ensureDevice(ctx)
	if err != nil {
		return Allocation{}, err
	}
	reqBody := map[string]any{"slug": slug, "ports": ports}
	var resp struct {
		Slug   string `json:"slug"`
		Secret string `json:"secret"`
		Frps   struct {
			Addr  string `json:"addr"`
			Token string `json:"token"`
		} `json:"frps"`
		Proxies []AllocProxy `json:"proxies"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/allocate", token, reqBody, &resp); err != nil {
		return Allocation{}, err
	}
	return Allocation{
		Slug:      resp.Slug,
		FrpsAddr:  resp.Frps.Addr,
		FrpsToken: resp.Frps.Token,
		Secret:    resp.Secret,
		Proxies:   resp.Proxies,
	}, nil
}

// Release frees all public ports held for slug.
func (c *Client) Release(ctx context.Context, slug string) error {
	token, err := c.ensureDevice(ctx)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPost, "/v1/release", token, map[string]any{"slug": slug}, nil)
}

// doJSON performs a JSON request to the control-plane. A non-empty bearer is
// sent as Authorization; out (if non-nil) receives the decoded response.
func (c *Client) doJSON(ctx context.Context, method, path, bearer string, body, out any) error {
	var rdr *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return fmt.Errorf("control-plane %s %s: %s (%d)", method, path, e.Error, resp.StatusCode)
		}
		return fmt.Errorf("control-plane %s %s: status %d", method, path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

package sync

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.8labs.io/jsuresh/note/internal/auth"
)

// Client talks to the note HTTP API with per-request signing.
type Client struct {
	BaseURL string
	User    string
	Priv    ed25519.PrivateKey
	HTTP    *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

// Register posts an unsigned registration request.
func Register(baseURL, user, adminPassword string, pub ed25519.PublicKey) error {
	body, err := auth.RegisterJSON(user, pub, adminPassword)
	if err != nil {
		return err
	}
	endpoint := joinURL(baseURL, "/v1/register")
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("register: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// RemoteNote is one row from GET /v1/notes.
type RemoteNote struct {
	Name    string `json:"name"`
	ModUnix int64  `json:"mod_time_unix"`
	Size    int64  `json:"size"`
}

// ListNotes returns the remote manifest.
func (c *Client) ListNotes() ([]RemoteNote, error) {
	const p = "/v1/notes"
	resp, err := c.signedRequest(http.MethodGet, p, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list notes: status %d: %s", resp.StatusCode, string(b))
	}
	var notes []RemoteNote
	if err := json.Unmarshal(b, &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// GetNote downloads note bytes (slug without .txt).
func (c *Client) GetNote(slug string) ([]byte, error) {
	p := "/v1/notes/" + url.PathEscape(slug)
	resp, err := c.signedRequest(http.MethodGet, p, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get %s: status %d: %s", slug, resp.StatusCode, string(b))
	}
	return b, nil
}

// PutNote uploads note bytes.
func (c *Client) PutNote(slug string, body []byte) error {
	p := "/v1/notes/" + url.PathEscape(slug)
	resp, err := c.signedRequest(http.MethodPut, p, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("put %s: status %d: %s", slug, resp.StatusCode, string(rb))
	}
	return nil
}

// Whoami verifies signed requests against the server (optional endpoint).
func (c *Client) Whoami() error {
	const p = "/v1/whoami"
	resp, err := c.signedRequest(http.MethodGet, p, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("whoami: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *Client) signedRequest(method, urlPath string, body []byte) (*http.Response, error) {
	if body == nil {
		body = []byte{}
	}
	ts := time.Now().Unix()
	sig := auth.Sign(c.Priv, method, urlPath, body, ts)
	endpoint := joinURL(c.BaseURL, urlPath)
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-User", c.User)
	req.Header.Set("X-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-Signature", sig)
	if method == http.MethodPut || method == http.MethodPost {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	return c.httpClient().Do(req)
}

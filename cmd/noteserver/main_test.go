package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"code.8labs.io/jsuresh/note/internal/auth"
)

func signRequest(priv ed25519.PrivateKey, user, method, path string, body []byte) http.Header {
	h := make(http.Header)
	h.Set("X-User", user)
	unix := time.Now().Unix()
	h.Set("X-Timestamp", strconv.FormatInt(unix, 10))
	h.Set("X-Signature", auth.Sign(priv, method, path, body, unix))
	return h
}

func newTestServer(t *testing.T) (*server, ed25519.PrivateKey, func()) {
	t.Helper()
	dir, err := ioutil.TempDir("", "noteserver")
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s := &server{datadir: dir, adminPassword: "adminpw"}
	cleanup := func() { _ = os.RemoveAll(dir) }
	// register user "alice" directly on disk + pubkey file
	user := "alice"
	if err := os.MkdirAll(s.userDir(user), 0o755); err != nil {
		cleanup()
		t.Fatal(err)
	}
	keyMaterial := base64.StdEncoding.EncodeToString(pub)
	if err := ioutil.WriteFile(s.pubkeyPath(user), []byte(keyMaterial+"\n"), 0o644); err != nil {
		cleanup()
		t.Fatal(err)
	}
	return s, priv, cleanup
}

func TestRegister_wrongPassword(t *testing.T) {
	dir, err := ioutil.TempDir("", "noteserver")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	s := &server{datadir: dir, adminPassword: "right"}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	reg := registerRequest{User: "bob", PublicKey: base64.StdEncoding.EncodeToString(pub), Password: "wrong"}
	b, _ := json.Marshal(reg)
	req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	s.handleRegister(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRegister_happyPath(t *testing.T) {
	dir, err := ioutil.TempDir("", "noteserver")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	s := &server{datadir: dir, adminPassword: "adminpw"}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	reg := registerRequest{User: "carol", PublicKey: base64.StdEncoding.EncodeToString(pub), Password: "adminpw"}
	b, _ := json.Marshal(reg)
	req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	s.handleRegister(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201 got %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(s.pubkeyPath("carol")); err != nil {
		t.Fatal(err)
	}
	// duplicate
	req2 := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(b))
	rec2 := httptest.NewRecorder()
	s.handleRegister(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("want 409 got %d", rec2.Code)
	}
}

func TestNotes_roundTrip(t *testing.T) {
	s, priv, cleanup := newTestServer(t)
	defer cleanup()
	user := "alice"
	content := []byte("hello sync")

	putPath := "/v1/notes/topic"
	req := httptest.NewRequest(http.MethodPut, putPath, bytes.NewReader(content))
	req.Header = signRequest(priv, user, http.MethodPut, putPath, content)
	rec := httptest.NewRecorder()
	s.handleNoteItem(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("put: want 204 got %d %s", rec.Code, rec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/notes", nil)
	listReq.Header = signRequest(priv, user, http.MethodGet, "/v1/notes", nil)
	listRec := httptest.NewRecorder()
	s.handleNotesCollection(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: want 200 got %d %s", listRec.Code, listRec.Body.String())
	}
	var entries []noteEntry
	if err := json.NewDecoder(listRec.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "topic" {
		t.Fatalf("unexpected list: %+v", entries)
	}

	getPath := "/v1/notes/topic"
	getReq := httptest.NewRequest(http.MethodGet, getPath, nil)
	getReq.Header = signRequest(priv, user, http.MethodGet, getPath, nil)
	getRec := httptest.NewRecorder()
	s.handleNoteItem(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: want 200 got %d", getRec.Code)
	}
	if string(getRec.Body.Bytes()) != string(content) {
		t.Fatalf("body mismatch: %q", getRec.Body.String())
	}
	if getRec.Header().Get("Last-Modified") == "" {
		t.Fatal("missing Last-Modified")
	}
}

func TestNoteFileLayout(t *testing.T) {
	s, priv, cleanup := newTestServer(t)
	defer cleanup()
	user := "alice"
	putPath := "/v1/notes/topic"
	content := []byte("x")
	req := httptest.NewRequest(http.MethodPut, putPath, bytes.NewReader(content))
	req.Header = signRequest(priv, user, http.MethodPut, putPath, content)
	rec := httptest.NewRecorder()
	s.handleNoteItem(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatal(rec.Code)
	}
	p := filepath.Join(s.datadir, "users", user, "topic.txt")
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}

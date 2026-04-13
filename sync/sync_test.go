package sync

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"code.8labs.io/jsuresh/note/internal/auth"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

type fakeServer struct {
	mu      sync.Mutex
	datadir string
	keys    map[string]ed25519.PublicKey
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	d := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(d, 0755); err != nil {
		t.Fatal(err)
	}
	return &fakeServer{datadir: d, keys: map[string]ed25519.PublicKey{}}
}

func (s *fakeServer) userDir(user string) string {
	return filepath.Join(s.datadir, "users", user)
}

func (s *fakeServer) notePath(user, slug string) string {
	return filepath.Join(s.userDir(user), slug+".txt")
}

func (s *fakeServer) register(w http.ResponseWriter, r *http.Request) {
	var body auth.RegisterPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pub, err := auth.ParsePublicKeyBase64(body.PublicKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.keys[body.User] = pub
	s.mu.Unlock()
	if err := os.MkdirAll(s.userDir(body.User), 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *fakeServer) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.Header.Get("X-User")
		tsStr := r.Header.Get("X-Timestamp")
		sig := r.Header.Get("X-Signature")
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body = ioutil.NopCloser(bytes.NewReader(body))
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			http.Error(w, "bad timestamp", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		pub := s.keys[u]
		s.mu.Unlock()
		if len(pub) != ed25519.PublicKeySize {
			http.Error(w, "unknown user", http.StatusUnauthorized)
			return
		}
		if err := auth.Verify(pub, r.Method, r.URL.Path, body, ts, sig); err != nil {
			http.Error(w, "auth: "+err.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *fakeServer) signedRoutes(w http.ResponseWriter, r *http.Request) {
	u := r.Header.Get("X-User")
	switch {
	case r.URL.Path == "/v1/whoami" && r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"user":"` + u + `"}`))
	case r.URL.Path == "/v1/notes" && r.Method == http.MethodGet:
		s.listNotes(w, u)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/notes/"):
		slug := strings.TrimPrefix(r.URL.Path, "/v1/notes/")
		b, err := ioutil.ReadFile(s.notePath(u, slug))
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(b)
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/notes/"):
		slug := strings.TrimPrefix(r.URL.Path, "/v1/notes/")
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(s.userDir(u), 0755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := ioutil.WriteFile(s.notePath(u, slug), body, 0644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func (s *fakeServer) listNotes(w http.ResponseWriter, user string) {
	dir := s.userDir(user)
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var out []RemoteNote
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".txt")
		full := filepath.Join(dir, e.Name())
		fi, err := os.Stat(full)
		if err != nil {
			continue
		}
		out = append(out, RemoteNote{Name: slug, ModUnix: fi.ModTime().Unix(), Size: fi.Size()})
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(out); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func setTestHome(t *testing.T) func() {
	t.Helper()
	orig := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(home, "notes")
	if err := os.MkdirAll(notes, 0755); err != nil {
		t.Fatal(err)
	}
	dbpath := filepath.Join(notes, "note.db")
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE tokens (token text not null, document text not null, count integer, PRIMARY KEY(token, document));`)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Setenv("HOME", orig) }
}

func TestSyncRoundTrip(t *testing.T) {
	cleanup := setTestHome(t)
	defer cleanup()

	fs := newFakeServer(t)
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	fs.mu.Lock()
	fs.keys["alice"] = pub
	fs.mu.Unlock()
	if err := os.MkdirAll(fs.userDir("alice"), 0755); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/register", fs.register)
	mux.Handle("/v1/", fs.withAuth(http.HandlerFunc(fs.signedRoutes)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	notesDir := filepath.Join(os.Getenv("HOME"), "notes")
	local := filepath.Join(notesDir, "alpha.txt")
	if err := ioutil.WriteFile(local, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	cl := &Client{BaseURL: srv.URL, User: "alice", Priv: priv}
	if err := Run(cl, Options{NotesDir: notesDir, Truth: TruthServer}); err != nil {
		t.Fatal(err)
	}
	b, err := ioutil.ReadFile(fs.notePath("alice", "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Fatalf("remote got %q", b)
	}

	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if err := Run(cl, Options{NotesDir: notesDir, Truth: TruthServer}); err != nil {
		t.Fatal(err)
	}
	b2, err := ioutil.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	if string(b2) != "hello" {
		t.Fatalf("local got %q", b2)
	}
}

func TestRegisterClient(t *testing.T) {
	cleanup := setTestHome(t)
	defer cleanup()

	fs := newFakeServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/register", fs.register)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	notesDir := filepath.Join(os.Getenv("HOME"), "notes")
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		t.Fatal(err)
	}
	priv, err := auth.EnsureKeyPair(notesDir)
	if err != nil {
		t.Fatal(err)
	}
	pub := priv.Public().(ed25519.PublicKey)
	if err := Register(srv.URL, "bob", "adminpw", pub); err != nil {
		t.Fatal(err)
	}
	fs.mu.Lock()
	_, ok := fs.keys["bob"]
	fs.mu.Unlock()
	if !ok {
		t.Fatal("expected registered key")
	}
}

package main

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.8labs.io/jsuresh/note/internal/auth"
)

var userNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}$`)
var noteNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,126}$`)

type registerRequest struct {
	User       string `json:"user"`
	PublicKey  string `json:"public_key"`
	Password   string `json:"password"`
}

type noteEntry struct {
	Name    string `json:"name"`
	ModTime string `json:"mod_time"`
	Size    int64  `json:"size"`
}

type errBody struct {
	Error string `json:"error"`
}

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	datadir := flag.String("datadir", "./note_data", "directory for user note files")
	adminPassword := flag.String("password", os.Getenv("NOTE_ADMIN_PASSWORD"), "admin secret for POST /v1/register (required; NOTE_ADMIN_PASSWORD env if unset here)")
	flag.Parse()
	if strings.TrimSpace(*adminPassword) == "" {
		fmt.Fprintln(os.Stderr, "noteserver: -password is required")
		os.Exit(2)
	}
	srv := &server{
		datadir:       *datadir,
		adminPassword: *adminPassword,
	}
	if err := os.MkdirAll(*datadir, 0o755); err != nil {
		log.Fatalf("datadir: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/register", srv.handleRegister)
	mux.HandleFunc("/v1/notes", srv.handleNotesCollection)
	mux.HandleFunc("/v1/notes/", srv.handleNoteItem)
	addr := *listen
	log.Printf("noteserver listening on %s datadir=%s", addr, *datadir)
	log.Fatal(http.ListenAndServe(addr, mux))
}

type server struct {
	datadir       string
	adminPassword string
}

func (s *server) userDir(user string) string {
	return filepath.Join(s.datadir, "users", user)
}

func (s *server) pubkeyPath(user string) string {
	return filepath.Join(s.userDir(user), "authorized_keys")
}

func (s *server) notePath(user, name string) string {
	return filepath.Join(s.userDir(user), name+".txt")
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "read body"})
		return
	}
	var req registerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "invalid json"})
		return
	}
	if !userNameRe.MatchString(req.User) {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "invalid user"})
		return
	}
	if !auth.SecurePasswordEqual(s.adminPassword, req.Password) {
		writeJSON(w, http.StatusUnauthorized, errBody{Error: "unauthorized"})
		return
	}
	pub, err := auth.ParseEd25519PublicKey(req.PublicKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "invalid public_key"})
		return
	}
	pkPath := s.pubkeyPath(req.User)
	if _, err := os.Stat(pkPath); err == nil {
		writeJSON(w, http.StatusConflict, errBody{Error: "user already exists"})
		return
	} else if !os.IsNotExist(err) {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "stat pubkey"})
		return
	}
	if err := os.MkdirAll(s.userDir(req.User), 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "mkdir"})
		return
	}
	// Store the same material the client registered (PEM or base64) for reload.
	if err := ioutil.WriteFile(pkPath, []byte(strings.TrimSpace(req.PublicKey)+"\n"), 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "write pubkey"})
		return
	}
	_ = pub // validated; file holds original encoding
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok", "user": req.User})
}

func (s *server) handleNotesCollection(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/notes" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleListNotes(w, r)
}

func (s *server) handleNoteItem(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/v1/notes/")
	name = strings.TrimSuffix(name, "/")
	if name == "" || strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleGetNote(w, r, name)
	case http.MethodPut:
		s.handlePutNote(w, r, name)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

var errNoUser = errors.New("no such user")

func (s *server) loadPubKey(user string) (ed25519.PublicKey, error) {
	b, err := ioutil.ReadFile(s.pubkeyPath(user))
	if err != nil {
		return nil, err
	}
	return auth.ParseEd25519PublicKey(string(b))
}

func (s *server) requireAuth(user string, r *http.Request, body []byte) error {
	pub, err := s.loadPubKey(user)
	if err != nil {
		if os.IsNotExist(err) {
			return errNoUser
		}
		return err
	}
	ts := strings.TrimSpace(r.Header.Get("X-Timestamp"))
	sig := strings.TrimSpace(r.Header.Get("X-Signature"))
	unix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return err
	}
	if err := auth.Verify(pub, r.Method, r.URL.Path, body, unix, sig); err != nil {
		return err
	}
	return nil
}

func (s *server) authError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNoUser) {
		writeJSON(w, http.StatusUnauthorized, errBody{Error: "unknown user"})
		return
	}
	writeJSON(w, http.StatusUnauthorized, errBody{Error: "unauthorized"})
}

func (s *server) handleListNotes(w http.ResponseWriter, r *http.Request) {
	user := strings.TrimSpace(r.Header.Get("X-User"))
	if !userNameRe.MatchString(user) {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "invalid X-User"})
		return
	}
	if err := s.requireAuth(user, r, nil); err != nil {
		s.authError(w, err)
		return
	}
	dir := s.userDir(user)
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []noteEntry{})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "list dir"})
		return
	}
	out := make([]noteEntry, 0)
	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}
		n := fi.Name()
		if !strings.HasSuffix(n, ".txt") {
			continue
		}
		base := strings.TrimSuffix(n, ".txt")
		if !noteNameRe.MatchString(base) {
			continue
		}
		out = append(out, noteEntry{
			Name:    base,
			ModTime: fi.ModTime().UTC().Format(time.RFC3339Nano),
			Size:    fi.Size(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleGetNote(w http.ResponseWriter, r *http.Request, name string) {
	user := strings.TrimSpace(r.Header.Get("X-User"))
	if !userNameRe.MatchString(user) || !noteNameRe.MatchString(name) {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "invalid user or note name"})
		return
	}
	if err := s.requireAuth(user, r, nil); err != nil {
		s.authError(w, err)
		return
	}
	p := s.notePath(user, name)
	b, err := ioutil.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "read note"})
		return
	}
	st, err := os.Stat(p)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "stat note"})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Last-Modified", st.ModTime().UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func (s *server) handlePutNote(w http.ResponseWriter, r *http.Request, name string) {
	user := strings.TrimSpace(r.Header.Get("X-User"))
	if !userNameRe.MatchString(user) || !noteNameRe.MatchString(name) {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "invalid user or note name"})
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "read body"})
		return
	}
	if err := s.requireAuth(user, r, body); err != nil {
		s.authError(w, err)
		return
	}
	p := s.notePath(user, name)
	if err := ioutil.WriteFile(p, body, 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "write note"})
		return
	}
	now := time.Now()
	if err := os.Chtimes(p, now, now); err != nil {
		// mtime may still reflect write; ignore best-effort
		_ = err
	}
	w.WriteHeader(http.StatusNoContent)
}

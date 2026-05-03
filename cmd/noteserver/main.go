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
	User            string `json:"user"`
	Password        string `json:"password"`
	UserPublicKey   string `json:"user_public_key"`
	DevicePublicKey string `json:"device_public_key"`
	PublicKey       string `json:"public_key"` // legacy: single key for identity + device
}

type registerDeviceRequest struct {
	User            string `json:"user"`
	Password        string `json:"password"`
	DevicePublicKey string `json:"device_public_key"`
}

type noteEntry struct {
	Name    string `json:"name"`
	ModTime string `json:"mod_time"`
	Size    int64  `json:"size"`
}

type errBody struct {
	Error string `json:"error"`
}

// wipeAllUsers removes datadir/users (every registered account and all synced notes), then recreates an empty users directory.
func wipeAllUsers(datadir string) error {
	usersDir := filepath.Join(datadir, "users")
	if err := os.RemoveAll(usersDir); err != nil {
		return err
	}
	return os.MkdirAll(usersDir, 0o755)
}

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	datadir := flag.String("datadir", "./note_data", "directory for user note files")
	adminPassword := flag.String("password", os.Getenv("NOTE_ADMIN_PASSWORD"), "admin secret for POST /v1/register (required; NOTE_ADMIN_PASSWORD env if unset here)")
	wipeAll := flag.Bool("wipe-all", false, "delete all users and notes under datadir/users, then exit (requires -password)")
	flag.Parse()
	if strings.TrimSpace(*adminPassword) == "" {
		fmt.Fprintln(os.Stderr, "noteserver: -password is required")
		os.Exit(2)
	}
	if err := os.MkdirAll(*datadir, 0o755); err != nil {
		log.Fatalf("datadir: %v", err)
	}
	if *wipeAll {
		if err := wipeAllUsers(*datadir); err != nil {
			log.Fatalf("wipe-all: %v", err)
		}
		abs, _ := filepath.Abs(*datadir)
		log.Printf("noteserver: wiped all users and notes under %s/users", abs)
		os.Exit(0)
	}
	srv := &server{
		datadir:       *datadir,
		adminPassword: *adminPassword,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/register", srv.handleRegister)
	mux.HandleFunc("/v1/register-device", srv.handleRegisterDevice)
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

func (s *server) identityPubPath(user string) string {
	return filepath.Join(s.userDir(user), "identity.pub")
}

func (s *server) authorizedDevicesPath(user string) string {
	return filepath.Join(s.userDir(user), "authorized_devices")
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
	if s.userExists(req.User) {
		writeJSON(w, http.StatusConflict, errBody{Error: "user already exists"})
		return
	}

	var userKeyLine, deviceKeyLine string
	if strings.TrimSpace(req.UserPublicKey) != "" && strings.TrimSpace(req.DevicePublicKey) != "" {
		if _, err := auth.ParseEd25519PublicKey(req.UserPublicKey); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody{Error: "invalid user_public_key"})
			return
		}
		if _, err := auth.ParseEd25519PublicKey(req.DevicePublicKey); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody{Error: "invalid device_public_key"})
			return
		}
		userKeyLine = strings.TrimSpace(req.UserPublicKey)
		deviceKeyLine = strings.TrimSpace(req.DevicePublicKey)
	} else if strings.TrimSpace(req.PublicKey) != "" {
		if _, err := auth.ParseEd25519PublicKey(req.PublicKey); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody{Error: "invalid public_key"})
			return
		}
		line := strings.TrimSpace(req.PublicKey)
		userKeyLine, deviceKeyLine = line, line
	} else {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "need user_public_key and device_public_key, or legacy public_key"})
		return
	}

	if err := os.MkdirAll(s.userDir(req.User), 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "mkdir"})
		return
	}
	if err := ioutil.WriteFile(s.identityPubPath(req.User), []byte(userKeyLine+"\n"), 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "write identity.pub"})
		return
	}
	if err := ioutil.WriteFile(s.authorizedDevicesPath(req.User), []byte(deviceKeyLine+"\n"), 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "write authorized_devices"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok", "user": req.User})
}

func (s *server) userExists(user string) bool {
	dir := s.userDir(user)
	if _, err := os.Stat(filepath.Join(dir, "identity.pub")); err == nil {
		return true
	}
	if _, err := os.Stat(s.authorizedDevicesPath(user)); err == nil {
		return true
	}
	if _, err := os.Stat(s.pubkeyPath(user)); err == nil {
		return true
	}
	return false
}

func (s *server) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "read body"})
		return
	}
	var req registerDeviceRequest
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
	newPub, err := auth.ParseEd25519PublicKey(req.DevicePublicKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody{Error: "invalid device_public_key"})
		return
	}
	if err := s.ensureUserMigrated(req.User); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "migrate keys"})
		return
	}
	if _, err := s.loadIdentityPub(req.User); err != nil {
		writeJSON(w, http.StatusNotFound, errBody{Error: "unknown user"})
		return
	}
	devPath := s.authorizedDevicesPath(req.User)
	existing, err := s.loadDevicePubkeys(req.User)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "read devices"})
		return
	}
	for _, p := range existing {
		if string(p) == string(newPub) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "note": "device already registered"})
			return
		}
	}
	f, err := os.OpenFile(devPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "open authorized_devices"})
		return
	}
	line := strings.TrimSpace(req.DevicePublicKey) + "\n"
	if _, err := f.WriteString(line); err != nil {
		_ = f.Close()
		writeJSON(w, http.StatusInternalServerError, errBody{Error: "append device"})
		return
	}
	_ = f.Close()
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

func (s *server) ensureUserMigrated(user string) error {
	dir := s.userDir(user)
	idPath := s.identityPubPath(user)
	devPath := s.authorizedDevicesPath(user)
	legPath := s.pubkeyPath(user)

	hasID := false
	if _, err := os.Stat(idPath); err == nil {
		hasID = true
	} else if !os.IsNotExist(err) {
		return err
	}
	hasDev := false
	if _, err := os.Stat(devPath); err == nil {
		hasDev = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if hasID && hasDev {
		return nil
	}
	b, err := ioutil.ReadFile(legPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	line := strings.TrimSpace(string(b))
	if line == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if !hasID {
		if err := ioutil.WriteFile(idPath, []byte(line+"\n"), 0o644); err != nil {
			return err
		}
	}
	if !hasDev {
		if err := ioutil.WriteFile(devPath, []byte(line+"\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) loadIdentityPub(user string) (ed25519.PublicKey, error) {
	if err := s.ensureUserMigrated(user); err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(s.identityPubPath(user))
	if err != nil {
		return nil, err
	}
	return auth.ParseEd25519PublicKey(string(b))
}

func (s *server) loadDevicePubkeys(user string) ([]ed25519.PublicKey, error) {
	if err := s.ensureUserMigrated(user); err != nil {
		return nil, err
	}
	devPath := s.authorizedDevicesPath(user)
	b, err := ioutil.ReadFile(devPath)
	if err != nil {
		if os.IsNotExist(err) {
			leg, err2 := ioutil.ReadFile(s.pubkeyPath(user))
			if err2 != nil {
				return nil, err
			}
			line := strings.TrimSpace(string(leg))
			if line == "" {
				return nil, err
			}
			pub, err3 := auth.ParseEd25519PublicKey(line)
			if err3 != nil {
				return nil, err3
			}
			return []ed25519.PublicKey{pub}, nil
		}
		return nil, err
	}
	var out []ed25519.PublicKey
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pub, err := auth.ParseEd25519PublicKey(line)
		if err != nil {
			return nil, err
		}
		out = append(out, pub)
	}
	if len(out) == 0 {
		return nil, os.ErrNotExist
	}
	return out, nil
}

func (s *server) requireAuth(user string, r *http.Request, body []byte) error {
	pubs, err := s.loadDevicePubkeys(user)
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
	for _, pub := range pubs {
		if auth.Verify(pub, r.Method, r.URL.Path, body, unix, sig) == nil {
			return nil
		}
	}
	return errors.New("invalid signature")
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

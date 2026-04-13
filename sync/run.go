package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"code.8labs.io/jsuresh/note/analyze"
	"code.8labs.io/jsuresh/note/internal/merge"
	"code.8labs.io/jsuresh/note/internal/paths"
	"code.8labs.io/jsuresh/note/internal/syncstate"
)

// TruthMode selects conflict resolution when local and remote both diverged.
type TruthMode string

const (
	TruthMerge     TruthMode = "merge"
	TruthServer    TruthMode = "server"
	TruthClient    TruthMode = "client"
	TruthLastWrite TruthMode = "lastwrite"
)

var slugRE = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Options configures a sync run.
type Options struct {
	NotesDir  string
	Truth     TruthMode
	Touched   map[string]struct{}
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hashFile(p string) (string, error) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return "", err
	}
	return hashBytes(b), nil
}

func localSlugs(notesDir string) (map[string]bool, error) {
	files, err := ioutil.ReadDir(notesDir)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if !strings.HasSuffix(name, ".txt") || strings.HasPrefix(name, ".") {
			continue
		}
		slug := strings.TrimSuffix(name, ".txt")
		if !slugRE.MatchString(slug) {
			continue
		}
		out[slug] = true
	}
	return out, nil
}

func remoteMap(list []RemoteNote) map[string]RemoteNote {
	m := make(map[string]RemoteNote)
	for _, n := range list {
		m[n.Name] = n
	}
	return m
}

// Run performs bidirectional sync using manifest + on-disk state.
func Run(c *Client, opts Options) error {
	if opts.Touched == nil {
		opts.Touched = map[string]struct{}{}
	}
	if err := os.MkdirAll(filepath.Join(opts.NotesDir, paths.SyncBaseDir), 0755); err != nil {
		return err
	}
	st, err := syncstate.Load(opts.NotesDir)
	if err != nil {
		return err
	}

	manifest, err := c.ListNotes()
	if err != nil {
		return err
	}
	rmap := remoteMap(manifest)

	locals, err := localSlugs(opts.NotesDir)
	if err != nil {
		return err
	}

	names := map[string]bool{}
	for k := range locals {
		names[k] = true
	}
	for k := range rmap {
		names[k] = true
	}
	order := make([]string, 0, len(names))
	for k := range names {
		order = append(order, k)
	}
	sort.Strings(order)

	for _, slug := range order {
		rn, hasRemote := rmap[slug]
		if err := syncOne(c, opts, st, slug, locals[slug], hasRemote, rn); err != nil {
			return fmt.Errorf("%s: %w", slug, err)
		}
	}

	manifest2, err := c.ListNotes()
	if err != nil {
		return err
	}
	r2 := remoteMap(manifest2)

	for slug := range opts.Touched {
		ds := st.Documents[slug]
		if ds == nil {
			ds = &syncstate.Doc{}
			st.Documents[slug] = ds
		}
		np := paths.NoteFile(opts.NotesDir, slug)
		if _, err := os.Stat(np); err == nil {
			h, err := hashFile(np)
			if err != nil {
				return err
			}
			ds.BaseHash = h
			body, err := ioutil.ReadFile(np)
			if err != nil {
				return err
			}
			if err := ioutil.WriteFile(paths.SyncBaseFile(opts.NotesDir, slug), body, 0600); err != nil {
				return err
			}
		} else if os.IsNotExist(err) {
			delete(st.Documents, slug)
			_ = os.Remove(paths.SyncBaseFile(opts.NotesDir, slug))
			continue
		} else {
			return err
		}
		if rn, ok := r2[slug]; ok {
			ds.RemoteMod = rn.ModUnix
			ds.RemoteSize = rn.Size
		} else {
			ds.RemoteMod = 0
			ds.RemoteSize = 0
		}
	}

	if err := syncstate.Save(opts.NotesDir, st); err != nil {
		return err
	}

	for slug := range opts.Touched {
		if err := analyze.DeleteTokensForDocument(slug); err != nil {
			return err
		}
		analyze.AnalyzeFile(slug)
	}
	return nil
}

func syncOne(c *Client, opts Options, st *syncstate.Root, slug string, hasLocal bool, hasRemote bool, rn RemoteNote) error {
	ds := st.Documents[slug]

	localPath := paths.NoteFile(opts.NotesDir, slug)
	var localHash string
	var localMod int64
	if hasLocal {
		fi, err := os.Stat(localPath)
		if err != nil {
			return err
		}
		localMod = fi.ModTime().Unix()
		h, err := hashFile(localPath)
		if err != nil {
			return err
		}
		localHash = h
	}

	remoteChanged := hasRemote && (ds == nil || rn.ModUnix != ds.RemoteMod || rn.Size != ds.RemoteSize)
	localChanged := hasLocal && (ds == nil || localHash != ds.BaseHash)

	// Restore local copy whenever the server has a note but the client file is missing.
	if !hasLocal && hasRemote {
		return download(c, opts, slug)
	}
	if !remoteChanged && !localChanged {
		return nil
	}

	switch {
	case hasLocal && !hasRemote:
		return upload(c, opts, slug)
	case hasLocal && hasRemote:
		if remoteChanged && !localChanged {
			return download(c, opts, slug)
		}
		if !remoteChanged && localChanged {
			return upload(c, opts, slug)
		}
		// both changed
		remoteBytes, err := c.GetNote(slug)
		if err != nil {
			return err
		}
		rh := hashBytes(remoteBytes)
		if localHash == rh {
			opts.Touched[slug] = struct{}{}
			return nil
		}
		final, err := resolveConflict(opts, slug, localPath, remoteBytes, ds, localMod, rn.ModUnix)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(localPath, final, 0644); err != nil {
			return err
		}
		opts.Touched[slug] = struct{}{}
		return nil
	}
	return nil
}

func download(c *Client, opts Options, slug string) error {
	b, err := c.GetNote(slug)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(paths.NoteFile(opts.NotesDir, slug), b, 0644); err != nil {
		return err
	}
	opts.Touched[slug] = struct{}{}
	return nil
}

func upload(c *Client, opts Options, slug string) error {
	b, err := ioutil.ReadFile(paths.NoteFile(opts.NotesDir, slug))
	if err != nil {
		return err
	}
	if err := c.PutNote(slug, b); err != nil {
		return err
	}
	opts.Touched[slug] = struct{}{}
	return nil
}

func resolveConflict(opts Options, slug, localPath string, remote []byte, ds *syncstate.Doc, localMod, remoteMod int64) ([]byte, error) {
	local, err := ioutil.ReadFile(localPath)
	if err != nil {
		return nil, err
	}
	switch opts.Truth {
	case TruthServer:
		return remote, nil
	case TruthClient:
		return local, nil
	case TruthLastWrite:
		if remoteMod > localMod {
			return remote, nil
		}
		if remoteMod < localMod {
			return local, nil
		}
		return remote, nil
	case TruthMerge:
		base := loadBaseBytes(opts.NotesDir, slug, ds)
		if len(base) == 0 {
			return pickLastWrite(local, remote, localMod, remoteMod)
		}
		out, err := merge.Diff3(local, base, remote)
		if err != nil {
			return pickLastWrite(local, remote, localMod, remoteMod)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown truth mode %q", opts.Truth)
	}
}

func pickLastWrite(local, remote []byte, localMod, remoteMod int64) ([]byte, error) {
	if remoteMod > localMod {
		return remote, nil
	}
	if remoteMod < localMod {
		return local, nil
	}
	return remote, nil
}

func loadBaseBytes(notesDir, slug string, ds *syncstate.Doc) []byte {
	if ds == nil || ds.BaseHash == "" {
		return nil
	}
	p := paths.SyncBaseFile(notesDir, slug)
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil
	}
	if hashBytes(b) != ds.BaseHash {
		return nil
	}
	return b
}

// ReindexNote deletes tokens and re-analyzes one document (slug without .txt).
func ReindexNote(slug string) error {
	if err := analyze.DeleteTokensForDocument(slug); err != nil {
		return err
	}
	analyze.AnalyzeFile(slug)
	return nil
}

// ParseTruth normalizes the CLI flag value.
func ParseTruth(s string) (TruthMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "merge":
		return TruthMerge, nil
	case "server":
		return TruthServer, nil
	case "client":
		return TruthClient, nil
	case "lastwrite":
		return TruthLastWrite, nil
	default:
		return "", fmt.Errorf("invalid --truth %q (merge|server|client|lastwrite)", s)
	}
}

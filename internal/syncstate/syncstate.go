package syncstate

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"code.8labs.io/jsuresh/note/internal/paths"
)

// Doc holds per-document sync metadata.
type Doc struct {
	BaseHash   string `json:"base_hash"`
	RemoteMod  int64  `json:"remote_mod_unix"`
	RemoteSize int64  `json:"remote_size"`
}

// Root is the on-disk sync state for the client.
type Root struct {
	Documents map[string]*Doc `json:"documents"`
}

func Load(notesDir string) (*Root, error) {
	p := paths.SyncStatePath(notesDir)
	b, err := ioutil.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Root{Documents: map[string]*Doc{}}, nil
		}
		return nil, err
	}
	var r Root
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if r.Documents == nil {
		r.Documents = map[string]*Doc{}
	}
	return &r, nil
}

func Save(notesDir string, r *Root) error {
	if r.Documents == nil {
		r.Documents = map[string]*Doc{}
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := paths.SyncStatePath(notesDir) + ".tmp"
	if err := ioutil.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, paths.SyncStatePath(notesDir))
}

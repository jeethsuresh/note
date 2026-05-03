package paths

import (
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
)

const (
	// Legacy single keypair filenames (migration only).
	PrivKeyFile = "note_id_ed25519"
	PubKeyFile  = "note_id_ed25519.pub"

	UserPrivKeyFile   = "note_user_ed25519"
	UserPubKeyFile    = "note_user_ed25519.pub"
	DevicePrivKeyFile = "note_device_ed25519"
	DevicePubKeyFile  = "note_device_ed25519.pub"

	SyncState = ".sync_state.json"
	SyncBaseDir  = ".sync_base"
	TrashDirName = ".trash"
)

// NotesDir returns $HOME/notes (no trailing slash).
func NotesDir() (string, error) {
	h, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "notes"), nil
}

func DBPath(notesDir string) string {
	return filepath.Join(notesDir, "note.db")
}

func KeyPaths(notesDir string) (priv, pub string) {
	return filepath.Join(notesDir, PrivKeyFile), filepath.Join(notesDir, PubKeyFile)
}

func UserKeyPaths(notesDir string) (priv, pub string) {
	return filepath.Join(notesDir, UserPrivKeyFile), filepath.Join(notesDir, UserPubKeyFile)
}

func DeviceKeyPaths(notesDir string) (priv, pub string) {
	return filepath.Join(notesDir, DevicePrivKeyFile), filepath.Join(notesDir, DevicePubKeyFile)
}

func SyncStatePath(notesDir string) string {
	return filepath.Join(notesDir, SyncState)
}

func SyncBaseFile(notesDir, slug string) string {
	return filepath.Join(notesDir, SyncBaseDir, slug+".txt")
}

func NoteFile(notesDir, slug string) string {
	return filepath.Join(notesDir, slug+".txt")
}

// TrashDir returns the per-machine trash folder under the notes directory.
func TrashDir(notesDir string) string {
	return filepath.Join(notesDir, TrashDirName)
}

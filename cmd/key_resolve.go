package cmd

import (
	"crypto/ed25519"
	"crypto/subtle"
	"fmt"
	"os"
	"strings"

	"code.8labs.io/jsuresh/note/internal/auth"
	"code.8labs.io/jsuresh/note/internal/paths"
	"github.com/spf13/viper"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func verifyOptionalPubAgainst(priv ed25519.PrivateKey, viperKey string) error {
	s := strings.TrimSpace(viper.GetString(viperKey))
	if s == "" {
		return nil
	}
	want, err := auth.ParsePublicKeyBase64(s)
	if err != nil {
		return fmt.Errorf("%s: %w", viperKey, err)
	}
	got := priv.Public().(ed25519.PublicKey)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return fmt.Errorf("%s does not match private key", viperKey)
	}
	return nil
}

func verifyOptionalUserPub(priv ed25519.PrivateKey) error {
	if err := verifyOptionalPubAgainst(priv, "user_public_key"); err != nil {
		return err
	}
	if strings.TrimSpace(viper.GetString("user_private_key")) == "" {
		if err := verifyOptionalPubAgainst(priv, "public_key"); err != nil {
			return err
		}
	}
	return nil
}

// loadUserPrivate reads the stable identity key from ~/.note.yaml (user_private_key or legacy private_key),
// then note_user_ed25519, then legacy note_id_ed25519 when note_user is absent.
func loadUserPrivate(notesDir string) (ed25519.PrivateKey, bool, error) {
	s := strings.TrimSpace(viper.GetString("user_private_key"))
	if s == "" {
		s = strings.TrimSpace(viper.GetString("private_key"))
	}
	if s != "" {
		priv, err := auth.ParsePrivateKeyBase64(s)
		if err != nil {
			return nil, false, fmt.Errorf("user_private_key: %w", err)
		}
		if err := verifyOptionalUserPub(priv); err != nil {
			return nil, false, err
		}
		return priv, true, nil
	}
	up, _ := paths.UserKeyPaths(notesDir)
	if fileExists(up) {
		priv, err := auth.LoadPrivateKey(up)
		return priv, true, err
	}
	legacyPriv, _ := paths.KeyPaths(notesDir)
	if fileExists(legacyPriv) {
		priv, err := auth.LoadPrivateKey(legacyPriv)
		return priv, true, err
	}
	return nil, false, nil
}

// loadDevicePrivateFromFile reads the per-machine signing key from ~/notes/note_device_ed25519 only
// (never from config — device keys are regenerated per device / login when missing).
func loadDevicePrivateFromFile(notesDir string) (ed25519.PrivateKey, bool, error) {
	dp, _ := paths.DeviceKeyPaths(notesDir)
	if !fileExists(dp) {
		return nil, false, nil
	}
	priv, err := auth.LoadPrivateKey(dp)
	return priv, true, err
}

// resolveKeyPairs loads user identity + device signing keys. Device secrets exist only on disk.
// When allowGenerate is true (login), a missing device key is created; user identity is never read from device_* yaml.
func resolveKeyPairs(notesDir string, allowGenerate bool) (userPriv, devicePriv ed25519.PrivateKey, err error) {
	u, uOK, err := loadUserPrivate(notesDir)
	if err != nil {
		return nil, nil, err
	}
	d, dOK, err := loadDevicePrivateFromFile(notesDir)
	if err != nil {
		return nil, nil, err
	}

	if !uOK && !dOK {
		if !allowGenerate {
			return nil, nil, fmt.Errorf("missing user identity and device keys; run login")
		}
		_, u, err := ed25519.GenerateKey(nil)
		if err != nil {
			return nil, nil, err
		}
		_, d, err := ed25519.GenerateKey(nil)
		if err != nil {
			return nil, nil, err
		}
		return u, d, nil
	}
	if uOK && !dOK {
		if !allowGenerate {
			return nil, nil, fmt.Errorf("missing device key (%s); run login on this machine", paths.DevicePrivKeyFile)
		}
		_, d, err := ed25519.GenerateKey(nil)
		if err != nil {
			return nil, nil, err
		}
		return u, d, nil
	}
	if !uOK && dOK {
		if !allowGenerate {
			return nil, nil, fmt.Errorf("missing user identity key in config; copy ~/.note.yaml or run login")
		}
		_, u, err := ed25519.GenerateKey(nil)
		if err != nil {
			return nil, nil, err
		}
		return u, d, nil
	}
	return u, d, nil
}

func persistKeyFilesIfMissing(notesDir string, userPriv, devicePriv ed25519.PrivateKey) error {
	up, _ := paths.UserKeyPaths(notesDir)
	if !fileExists(up) {
		if err := auth.WriteUserKeyPair(notesDir, userPriv); err != nil {
			return err
		}
	}
	dp, _ := paths.DeviceKeyPaths(notesDir)
	if !fileExists(dp) {
		if err := auth.WriteDeviceKeyPair(notesDir, devicePriv); err != nil {
			return err
		}
	}
	return nil
}

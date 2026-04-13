package auth

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"code.8labs.io/jsuresh/note/internal/paths"
)

const maxClockSkew = 5 * time.Minute

// CanonicalString is the message signed for each authenticated request.
func CanonicalString(method, urlPath string, body []byte, unix int64) string {
	h := sha256.Sum256(body)
	return strings.ToUpper(method) + "\n" + urlPath + "\n" + hex.EncodeToString(h[:]) + "\n" + strconv.FormatInt(unix, 10)
}

// Sign produces base64-encoded Ed25519 signature for CanonicalString.
func Sign(priv ed25519.PrivateKey, method, urlPath string, body []byte, unix int64) string {
	msg := CanonicalString(method, urlPath, body, unix)
	sig := ed25519.Sign(priv, []byte(msg))
	return base64.StdEncoding.EncodeToString(sig)
}

// Verify checks a request signature using the user's public key.
func Verify(pub ed25519.PublicKey, method, urlPath string, body []byte, unix int64, sigB64 string) error {
	if len(pub) != ed25519.PublicKeySize {
		return errors.New("invalid public key")
	}
	ts := time.Unix(unix, 0)
	now := time.Now()
	if ts.Before(now.Add(-maxClockSkew)) || ts.After(now.Add(maxClockSkew)) {
		return fmt.Errorf("timestamp outside ±5m window")
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return err
	}
	msg := CanonicalString(method, urlPath, body, unix)
	if !ed25519.Verify(pub, []byte(msg), sig) {
		return errors.New("invalid signature")
	}
	return nil
}

// EnsureKeyPair creates an Ed25519 keypair beside the notes DB if missing.
func EnsureKeyPair(notesDir string) (ed25519.PrivateKey, error) {
	privPath, pubPath := paths.KeyPaths(notesDir)
	if _, err := os.Stat(privPath); os.IsNotExist(err) {
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(privPath, priv, 0600); err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(pubPath, pub, 0644); err != nil {
			return nil, err
		}
		return priv, nil
	}
	return LoadPrivateKey(privPath)
}

// LoadPrivateKey reads a 64-byte seed+pub private key file or 32-byte seed (extended on load).
func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) == ed25519.PrivateKeySize {
		return ed25519.PrivateKey(b), nil
	}
	if len(b) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(b), nil
	}
	return nil, fmt.Errorf("unexpected private key size %d", len(b))
}

// LoadPublicKey reads a raw 32-byte public key file.
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("unexpected public key size %d", len(b))
	}
	return ed25519.PublicKey(b), nil
}

// PublicKeyBase64 returns the standard base64 encoding of the raw public key bytes.
func PublicKeyBase64(pub ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(pub)
}

// ParsePublicKeyBase64 decodes a standard base64-encoded raw Ed25519 public key.
func ParsePublicKeyBase64(s string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("unexpected public key size %d", len(b))
	}
	return ed25519.PublicKey(b), nil
}

// RegisterPayload is the JSON body for POST /v1/register.
type RegisterPayload struct {
	User      string `json:"user"`
	PublicKey string `json:"public_key"`
	Password  string `json:"password"`
}

func RegisterJSON(user string, pub ed25519.PublicKey, adminPassword string) ([]byte, error) {
	p := RegisterPayload{
		User:      user,
		PublicKey: PublicKeyBase64(pub),
		Password:  adminPassword,
	}
	return json.Marshal(p)
}

package auth

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestCanonicalString_Deterministic(t *testing.T) {
	got := CanonicalString("GET", "/v1/notes", []byte("abc"), 42)
	want := "GET\n/v1/notes\n" + "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" + "\n42"
	if got != want {
		t.Fatalf("canonical mismatch:\n got:  %q\n want: %q", got, want)
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`hello`)
	ts := time.Now().Unix()
	sig := Sign(priv, "PUT", "/v1/notes/x", body, ts)
	if err := Verify(pub, "PUT", "/v1/notes/x", body, ts, sig); err != nil {
		t.Fatal(err)
	}
	if err := Verify(pub, "PUT", "/v1/notes/x", body, ts, sig+"x"); err == nil {
		t.Fatal("expected verify error")
	}
}

func TestPrivateKeyBase64RoundTrip(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	s := PrivateKeyBase64(priv)
	got, err := ParsePrivateKeyBase64(s)
	if err != nil {
		t.Fatal(err)
	}
	if subtle.ConstantTimeCompare(priv.Seed(), got.Seed()) != 1 {
		t.Fatal("seed mismatch after round trip")
	}
	full := base64.StdEncoding.EncodeToString(priv)
	got2, err := ParsePrivateKeyBase64(full)
	if err != nil {
		t.Fatal(err)
	}
	if subtle.ConstantTimeCompare(priv, got2) != 1 {
		t.Fatal("full private key mismatch after round trip")
	}
}

func TestRegisterJSONShape(t *testing.T) {
	pub := make(ed25519.PublicKey, ed25519.PublicKeySize)
	b, err := RegisterJSON("alice", pub, "secret")
	if err != nil {
		t.Fatal(err)
	}
	var p RegisterPayload
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if p.User != "alice" || p.Password != "secret" || p.PublicKey == "" {
		t.Fatalf("unexpected payload: %+v", p)
	}
}

func TestRegisterUserJSONShape(t *testing.T) {
	up := make(ed25519.PublicKey, ed25519.PublicKeySize)
	dp := make(ed25519.PublicKey, ed25519.PublicKeySize)
	dp[0] = 1
	b, err := RegisterUserJSON("alice", up, dp, "secret")
	if err != nil {
		t.Fatal(err)
	}
	var p RegisterPayload
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if p.User != "alice" || p.Password != "secret" || p.UserPublicKey == "" || p.DevicePublicKey == "" || p.PublicKey != "" {
		t.Fatalf("unexpected payload: %+v", p)
	}
}

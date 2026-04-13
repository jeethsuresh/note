package auth

import (
	"crypto/ed25519"
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

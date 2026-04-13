package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestSignVerify_roundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	method := "PUT"
	path := "/v1/notes/hello"
	body := []byte("note body")
	unix := time.Now().Unix()
	sig := Sign(priv, method, path, body, unix)
	if err := Verify(pub, method, path, body, unix, sig); err != nil {
		t.Fatal(err)
	}
	if err := Verify(pub, method, path, []byte("other"), unix, sig); err == nil {
		t.Fatal("expected verify failure on body change")
	}
}

func TestVerify_clockSkew(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	unix := time.Now().Unix() - int64((6 * time.Minute).Seconds())
	sig := Sign(priv, "GET", "/v1/notes", nil, unix)
	if err := Verify(pub, "GET", "/v1/notes", nil, unix, sig); err == nil {
		t.Fatal("expected skew failure")
	}
}

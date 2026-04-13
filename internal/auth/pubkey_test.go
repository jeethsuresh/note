package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

func TestParseEd25519PublicKey_rawBase64(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s := base64.StdEncoding.EncodeToString(pub)
	got, err := ParseEd25519PublicKey(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(pub) {
		t.Fatalf("pub mismatch")
	}
}

func TestParseEd25519PublicKey_PEM(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	blk := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	pemStr := string(pem.EncodeToMemory(blk))
	got, err := ParseEd25519PublicKey(pemStr)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(pub) {
		t.Fatalf("pub mismatch")
	}
}

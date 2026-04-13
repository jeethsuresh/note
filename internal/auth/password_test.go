package auth

import "testing"

func TestSecurePasswordEqual(t *testing.T) {
	if !SecurePasswordEqual("secret", "secret") {
		t.Fatal("expected match")
	}
	if SecurePasswordEqual("secret", "other") {
		t.Fatal("expected mismatch")
	}
}

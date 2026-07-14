package crypto

import (
	"encoding/base64"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	c, err := NewFromKey(key)
	if err != nil {
		t.Fatalf("NewFromKey failed: %v", err)
	}

	plaintext := []byte("Hello, World!")
	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("round-trip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptProducesDifferentOutput(t *testing.T) {
	key := make([]byte, 32)
	c, _ := NewFromKey(key)
	plaintext := []byte("same text")

	ct1, _ := c.Encrypt(plaintext)
	ct2, _ := c.Encrypt(plaintext)

	if string(ct1) == string(ct2) {
		t.Error("encrypt should produce non-deterministic output due to random nonce")
	}
}

func TestDecryptTamperedData(t *testing.T) {
	key := make([]byte, 32)
	c, _ := NewFromKey(key)
	plaintext := []byte("tamper test")

	ct, _ := c.Encrypt(plaintext)
	ct[len(ct)/2] ^= 0xFF

	_, err := c.Decrypt(ct)
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed on tampered data, got %v", err)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key1[0] = 1
	key2 := make([]byte, 32)
	key2[0] = 2

	c1, _ := NewFromKey(key1)
	c2, _ := NewFromKey(key2)

	ct, _ := c1.Encrypt([]byte("test"))
	_, err := c2.Decrypt(ct)
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed with wrong key, got %v", err)
	}
}

func TestEncryptEmptyInput(t *testing.T) {
	key := make([]byte, 32)
	c, _ := NewFromKey(key)

	ct, err := c.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Encrypt empty failed: %v", err)
	}
	dec, err := c.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt empty failed: %v", err)
	}
	if len(dec) != 0 {
		t.Errorf("expected empty plaintext, got %v", dec)
	}
}

func TestNewInvalidKeyLength(t *testing.T) {
	_, err := NewFromKey(make([]byte, 16))
	if err != ErrInvalidKey {
		t.Errorf("expected ErrInvalidKey for 16-byte key, got %v", err)
	}
}

func TestNewBase64Key(t *testing.T) {
	key := make([]byte, 32)
	b64 := base64.StdEncoding.EncodeToString(key)
	c, err := New(b64)
	if err != nil {
		t.Fatalf("New with base64 key failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil Crypto")
	}
}

func TestHash(t *testing.T) {
	h1 := Hash("test@example.com")
	h2 := Hash("test@example.com")
	h3 := Hash("other@example.com")

	if h1 != h2 {
		t.Error("Hash should be deterministic")
	}
	if h1 == h3 {
		t.Error("Different inputs should produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(h1))
	}
}

package crypto

import (
	"encoding/hex"
	"testing"
)

func TestNewEncryptor_ValidKey(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enc == nil {
		t.Fatal("expected non-nil encryptor")
	}
}

func TestNewEncryptor_WrongLength(t *testing.T) {
	_, err := NewEncryptor("tooshort")
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestNewEncryptor_InvalidHex(t *testing.T) {
	// 64 chars but not valid hex
	_, err := NewEncryptor("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestRoundTrip(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("creating encryptor: %v", err)
	}

	plaintext := []byte("sk-secret-api-key-12345")
	ciphertext, nonce, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypting: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext, nonce)
	if err != nil {
		t.Fatalf("decrypting: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestDistinctCiphertexts(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("creating encryptor: %v", err)
	}

	plaintext := []byte("same-input")
	ct1, n1, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("first encrypt: %v", err)
	}

	ct2, n2, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("second encrypt: %v", err)
	}

	if hex.EncodeToString(ct1) == hex.EncodeToString(ct2) {
		t.Error("two encryptions of the same plaintext produced identical ciphertext")
	}

	if hex.EncodeToString(n1) == hex.EncodeToString(n2) {
		t.Error("two encryptions produced identical nonces")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	key2 := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	enc1, _ := NewEncryptor(key1)
	enc2, _ := NewEncryptor(key2)

	ciphertext, nonce, err := enc1.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("encrypting: %v", err)
	}

	_, err = enc2.Decrypt(ciphertext, nonce)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestDecrypt_CorruptedCiphertext(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enc, _ := NewEncryptor(key)

	ciphertext, nonce, _ := enc.Encrypt([]byte("secret"))
	ciphertext[0] ^= 0xff // corrupt first byte

	_, err := enc.Decrypt(ciphertext, nonce)
	if err == nil {
		t.Fatal("expected error for corrupted ciphertext")
	}
}

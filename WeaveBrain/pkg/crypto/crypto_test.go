package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := []byte("a very very very very secret key") // 32 bytes
	plaintext := []byte("hello world, this is a secret message for MCP")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Fatal("Ciphertext should not equal plaintext")
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("Decrypted text does not match original plaintext. Got %q, want %q", decrypted, plaintext)
	}
}

func TestInvalidKeySize(t *testing.T) {
	key := []byte("too short")
	plaintext := []byte("hello")

	_, err := Encrypt(plaintext, key)
	if err != ErrInvalidKeySize {
		t.Fatalf("Expected ErrInvalidKeySize, got %v", err)
	}

	_, err = Decrypt([]byte("some ciphertext"), key)
	if err != ErrInvalidKeySize {
		t.Fatalf("Expected ErrInvalidKeySize, got %v", err)
	}
}

func TestCiphertextTooShort(t *testing.T) {
	key := []byte("a very very very very secret key")
	ciphertext := []byte("short")

	_, err := Decrypt(ciphertext, key)
	if err != ErrCiphertextTooShort {
		t.Fatalf("Expected ErrCiphertextTooShort, got %v", err)
	}
}

func TestTamperedCiphertext(t *testing.T) {
	key := []byte("a very very very very secret key")
	plaintext := []byte("hello world")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with the ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xff

	_, err = Decrypt(ciphertext, key)
	if err == nil {
		t.Fatal("Expected decryption to fail for tampered ciphertext, but it succeeded")
	}
}

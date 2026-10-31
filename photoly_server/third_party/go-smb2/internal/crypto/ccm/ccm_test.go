package ccm

import (
	"bytes"
	"crypto/aes"
	"testing"
)

func TestCCMSealOpenAndPayloadLimit(t *testing.T) {
	block, err := aes.NewCipher(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	aead, err := NewCCMWithNonceAndTagSizes(block, 13, 16)
	if err != nil {
		t.Fatal(err)
	}
	nonce, data := make([]byte, 13), []byte("authenticated header")
	plain := []byte("SMB encrypted payload")
	encrypted := aead.Seal(nil, nonce, plain, data)
	if len(encrypted) != len(plain)+aead.Overhead() {
		t.Fatalf("ciphertext length = %d", len(encrypted))
	}
	decoded, err := aead.Open(nil, nonce, encrypted, data)
	if err != nil || !bytes.Equal(decoded, plain) {
		t.Fatalf("round trip: %q %v", decoded, err)
	}
	encrypted[0] ^= 1
	if _, err := aead.Open(nil, nonce, encrypted, data); err == nil {
		t.Fatal("accepted modified ciphertext")
	}
	if encrypted := aead.Seal(nil, nonce, make([]byte, 1<<16), data); encrypted != nil {
		t.Fatal("accepted payload beyond nonce-size limit")
	}
}

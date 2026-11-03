package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"os"
)

var ErrNoSecretKey = errors.New("STORAGE_SECRET_KEY must be set to save storage credentials")

func gcm() (cipher.AEAD, error) {
	return gcmKey(os.Getenv("STORAGE_SECRET_KEY"))
}

func gcmKey(k string) (cipher.AEAD, error) {
	if k == "" {
		return nil, ErrNoSecretKey
	}
	sum := sha256.Sum256([]byte(k))
	b, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

func seal(plain []byte) ([]byte, error) {
	g, err := gcm()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plain, nil), nil
}

func open(sealed []byte) ([]byte, error) {
	g, err := gcm()
	if err != nil {
		return nil, err
	}
	if len(sealed) < g.NonceSize() {
		return nil, errors.New("storage: corrupt secret")
	}
	return g.Open(nil, sealed[:g.NonceSize()], sealed[g.NonceSize():], nil)
}

func SecretKeyConfigured() bool { return os.Getenv("STORAGE_SECRET_KEY") != "" }

// ReencryptSecret migrates credentials without changing process environment or live configuration.
func ReencryptSecret(sealed []byte, oldKey, newKey string) ([]byte, error) {
	if len(sealed) == 0 {
		return nil, nil
	}
	old, err := gcmKey(oldKey)
	if err != nil {
		return nil, err
	}
	if len(sealed) < old.NonceSize() {
		return nil, errors.New("storage: corrupt secret")
	}
	plain, err := old.Open(nil, sealed[:old.NonceSize()], sealed[old.NonceSize():], nil)
	if err != nil {
		return nil, err
	}
	next, err := gcmKey(newKey)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, next.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return next.Seal(nonce, nonce, plain, nil), nil
}

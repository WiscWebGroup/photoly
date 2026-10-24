package legacy

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"
)

const saltChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// HashPassword is werkzeug.security.generate_password_hash (scrypt:32768:8:1).
func HashPassword(password string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = saltChars[int(b[i])%len(saltChars)]
	}
	salt := string(b)
	key, _ := scrypt.Key([]byte(password), []byte(salt), 32768, 8, 1, 64)
	return "scrypt:32768:8:1$" + salt + "$" + hex.EncodeToString(key)
}

// CheckPassword is werkzeug.security.check_password_hash for scrypt and pbkdf2:sha256 hashes.
func CheckPassword(hash, password string) bool {
	parts := strings.SplitN(hash, "$", 3)
	if len(parts) != 3 {
		return false
	}
	method, salt, want := strings.Split(parts[0], ":"), parts[1], parts[2]
	var got []byte
	switch method[0] {
	case "scrypt":
		n, r, p := 1<<15, 8, 1
		if len(method) == 4 {
			a, e1 := strconv.Atoi(method[1])
			b, e2 := strconv.Atoi(method[2])
			c, e3 := strconv.Atoi(method[3])
			if e1 != nil || e2 != nil || e3 != nil {
				return false
			}
			n, r, p = a, b, c
		}
		k, err := scrypt.Key([]byte(password), []byte(salt), n, r, p, 64)
		if err != nil {
			return false
		}
		got = k
	case "pbkdf2":
		if len(method) < 2 || method[1] != "sha256" {
			return false
		}
		iter := 600000
		if len(method) > 2 {
			v, err := strconv.Atoi(method[2])
			if err != nil {
				return false
			}
			iter = v
		}
		got = pbkdf2.Key([]byte(password), []byte(salt), iter, 32, sha256.New)
	default:
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(got)), []byte(want)) == 1
}

package auth

import (
	"crypto/rand"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

func GenerateAPIKey() (plaintext, prefix, hash string) {
	raw := make([]byte, 32)
	rand.Read(raw)
	plaintext = "sk-" + hex.EncodeToString(raw)
	prefix = plaintext[:8]
	h, _ := bcrypt.GenerateFromPassword([]byte(plaintext), 10)
	hash = string(h)
	return
}

func VerifyAPIKey(plaintext, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}

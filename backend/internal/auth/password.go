package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	Argon2MemoryKiB   uint32 = 64 * 1024
	Argon2Iterations  uint32 = 3
	Argon2Parallelism uint8  = 2
	Argon2SaltBytes          = 16
	Argon2KeyBytes           = 32
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, Argon2SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, Argon2Iterations, Argon2MemoryKiB, Argon2Parallelism, Argon2KeyBytes)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", Argon2MemoryKiB, Argon2Iterations, Argon2Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(encodedHash, password string) bool {
	segments := strings.Split(encodedHash, "$")
	if len(segments) != 6 || segments[1] != "argon2id" || segments[2] != "v=19" {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(segments[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil || memory == 0 || iterations == 0 || parallelism == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(segments[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(segments[5])
	if err != nil || len(expected) == 0 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

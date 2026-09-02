package auth

import "testing"

func TestPasswordHashVerification(t *testing.T) {
	hash, err := HashPassword("a sufficient operator password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !VerifyPassword(hash, "a sufficient operator password") {
		t.Fatal("expected password verification to succeed")
	}
	if VerifyPassword(hash, "incorrect password") {
		t.Fatal("expected password verification to fail")
	}
}

func TestArgon2MemoryCeiling(t *testing.T) {
	const allowedMemoryKiB = 64 * 1024
	if Argon2MemoryKiB > allowedMemoryKiB {
		t.Fatalf("argon2 memory %d KiB exceeds %d KiB", Argon2MemoryKiB, allowedMemoryKiB)
	}
}

func TestPasswordMinimumLength(t *testing.T) {
	if ValidatePassword("short") == nil {
		t.Fatal("expected short password rejection")
	}
	if err := ValidatePassword("twelve chars"); err != nil {
		t.Fatalf("expected valid password: %v", err)
	}
}

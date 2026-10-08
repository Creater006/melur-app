package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	password := "MelurDemo123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !VerifyPassword(password, hash) {
		t.Fatal("VerifyPassword rejected the matching password")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Fatal("VerifyPassword accepted a different password")
	}
}

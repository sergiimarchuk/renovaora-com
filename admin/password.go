package main

import "golang.org/x/crypto/bcrypt"

// bcryptCost controls how expensive hashing is. Higher = slower = more
// resistant to brute-force, at the cost of CPU time per login attempt.
// 12 is a reasonable modern default (14 is noticeably slower; fine for a
// single admin logging in occasionally, but 12 balances security and
// responsiveness well for this use case).
const bcryptCost = 12

// HashPassword returns a bcrypt hash of the given plaintext password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
// Returns true only if they match.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

package service

import "golang.org/x/crypto/bcrypt"

// hashPassword hashes a plain-text password using bcrypt at cost 12.
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

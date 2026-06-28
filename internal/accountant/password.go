package accountant

import (
	"crypto/rand"
	"math/big"
)

const passwordAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}<>?,."

// GeneratePassword returns a cryptographically random password of the given length.
func GeneratePassword(length int) (string, error) {
	n := big.NewInt(int64(len(passwordAlphabet)))
	b := make([]byte, length)
	for i := range b {
		idx, err := rand.Int(rand.Reader, n)
		if err != nil {
			return "", err
		}
		b[i] = passwordAlphabet[idx.Int64()]
	}
	return string(b), nil
}

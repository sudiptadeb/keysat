package capture

import (
	"crypto/sha256"
	"fmt"
	"unicode"
)

// IsLikelyPassword returns true if the word looks like a password.
// A word is considered password-like when it has 8+ characters and
// satisfies at least 3 of 4 character-class checks: uppercase,
// lowercase, digit, special character.
// This is the secondary heuristic; the primary check is secure-input
// detection at the platform level.
func IsLikelyPassword(word string) bool {
	if len([]rune(word)) < 8 {
		return false
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range word {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	score := 0
	if hasUpper {
		score++
	}
	if hasLower {
		score++
	}
	if hasDigit {
		score++
	}
	if hasSpecial {
		score++
	}

	return score >= 3
}

// HashWord returns a SHA-256 hex digest of the word, prefixed with "sha256:".
func HashWord(word string) string {
	h := sha256.Sum256([]byte(word))
	return fmt.Sprintf("sha256:%x", h)
}

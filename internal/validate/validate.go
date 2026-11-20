// Package validate performs local argument checks shared by the
// family clients. Every failure is returned as a plain error; callers
// wrap it into a *fastnearerr.Error with Code "validation_error".
package validate

import (
	"errors"
	"fmt"
	"strings"
)

// publicKeyPrefixes are the accepted NEAR public-key type tags.
var publicKeyPrefixes = []string{
	"ed25519:",
	"secp256k1:",
	"ml-dsa-65:",
	"ml-dsa-65-hash:",
}

// AccountID checks a NEAR account id: non-empty, at most 64
// characters, no whitespace and no '/'.
func AccountID(id string) error {
	if id == "" {
		return errors.New("account_id must not be empty")
	}
	if len(id) > 64 {
		return fmt.Errorf("account_id must be at most 64 characters, got %d", len(id))
	}
	if strings.ContainsAny(id, " \t\r\n/") {
		return errors.New("account_id must not contain whitespace or '/'")
	}
	return nil
}

// PublicKey checks the type prefix of a NEAR public key
// ("ed25519:...", "secp256k1:...", "ml-dsa-65:...",
// "ml-dsa-65-hash:...").
func PublicKey(pk string) error {
	if pk == "" {
		return errors.New("public_key must not be empty")
	}
	for _, p := range publicKeyPrefixes {
		if strings.HasPrefix(pk, p) && len(pk) > len(p) {
			return nil
		}
	}
	return fmt.Errorf("public_key must start with one of %s (use the full key, not a truncated form)",
		strings.Join(publicKeyPrefixes, ", "))
}

// Limit checks a pagination limit against [lo, hi].
func Limit(name string, v, lo, hi int) error {
	if v < lo || v > hi {
		return fmt.Errorf("%s must be in [%d, %d], got %d", name, lo, hi, v)
	}
	return nil
}

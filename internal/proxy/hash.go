package proxy

import (
	"crypto/sha256"
	"fmt"
)

func hashServiceID(id string, doHash bool) string {
	if !doHash || id == "" {
		return id
	}
	h := sha256.Sum256([]byte(id))
	return fmt.Sprintf("%x", h[:8]) // 16-char prefix is enough for a service registry key
}

// hashUser returns SHA-256(username + orgSalt) as a full hex string.
// The org_salt prevents cross-org correlation of the same username.
func hashUser(username, orgSalt string) string {
	h := sha256.Sum256([]byte(username + orgSalt))
	return fmt.Sprintf("%x", h[:])
}

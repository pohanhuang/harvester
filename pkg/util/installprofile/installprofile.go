// Package installprofile holds the cluster-wide, install-time decisions that a
// joining node must follow, and the token-based signing shared by the server
// and the installer.
package installprofile

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
)

const (
	ConfigMapName = "harvester-install-profile"
	// DeferLonghornInstallKey is the ConfigMap data key.
	DeferLonghornInstallKey = "deferLonghornInstall"

	Path        = "/v1/harvester/installProfile"
	NonceHeader = "X-Cattle-Nonce"
	HashHeader  = "X-Cattle-Hash"
)

// InstallProfile is written once at cluster creation and never updated.
type InstallProfile struct {
	DeferLonghornInstall bool `json:"deferLonghornInstall"`
}

// BearerToken is the value sent as "Authorization: Bearer <value>".
func BearerToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// Sign returns base64(HMAC-SHA512(token, nonce \0 body \0)).
func Sign(token, nonce string, body []byte) string {
	h := hmac.New(sha512.New, []byte(token))
	h.Write([]byte(nonce))
	h.Write([]byte{0})
	h.Write(body)
	h.Write([]byte{0})
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// Verify reports whether hash is the valid signature of nonce and body.
func Verify(token, nonce string, body []byte, hash string) bool {
	return hmac.Equal([]byte(Sign(token, nonce, body)), []byte(hash))
}

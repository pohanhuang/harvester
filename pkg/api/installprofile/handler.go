package installprofile

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/harvester/harvester/pkg/util/installprofile"
)

// Handler serves the cluster InstallProfile to a joining node. The request is
// authenticated with the join token and the response is signed with it, so a
// server that does not know the token cannot forge a profile.
type Handler struct {
	getToken  func() (string, error)
	getConfig func() (map[string]string, error)
}

// New builds a Handler. getToken returns the join token; getConfig returns the
// install profile ConfigMap data (nil, nil when the ConfigMap does not exist).
func New(getToken func() (string, error), getConfig func() (map[string]string, error)) *Handler {
	return &Handler{getToken: getToken, getConfig: getConfig}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nonce := r.Header.Get(installprofile.NonceHeader)
	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if nonce == "" || bearer == "" {
		http.Error(w, "nonce and bearer token required", http.StatusUnauthorized)
		return
	}

	token, err := h.getToken()
	if err != nil {
		logrus.WithError(err).Error("failed to get join token")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if subtle.ConstantTimeCompare([]byte(installprofile.BearerToken(token)), []byte(bearer)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	data, err := h.getConfig()
	if err != nil && !errors.IsNotFound(err) {
		logrus.WithError(err).Error("failed to get install profile")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	// A cluster created before this feature has no ConfigMap: Longhorn is installed.
	profile := installprofile.InstallProfile{
		DeferLonghornInstall: data[installprofile.DeferLonghornInstallKey] == "true",
	}
	body, err := json.Marshal(profile)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set(installprofile.HashHeader, installprofile.Sign(token, nonce, body))
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

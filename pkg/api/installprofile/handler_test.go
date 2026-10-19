package installprofile

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/harvester/harvester/pkg/util/installprofile"
)

func serve(t *testing.T, h *Handler, nonce, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, installprofile.Path, nil)
	if nonce != "" {
		req.Header.Set(installprofile.NonceHeader, nonce)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestHandler(t *testing.T) {
	const token = "secret"
	tokenFn := func() (string, error) { return token, nil }
	deferred := New(tokenFn, func() (map[string]string, error) {
		return map[string]string{installprofile.DeferLonghornInstallKey: "true"}, nil
	})

	rr := serve(t, deferred, "n1", installprofile.BearerToken(token))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"deferLonghornInstall":true}`, rr.Body.String())
	assert.True(t, installprofile.Verify(token, "n1", rr.Body.Bytes(), rr.Header().Get(installprofile.HashHeader)))

	assert.Equal(t, http.StatusUnauthorized, serve(t, deferred, "n1", installprofile.BearerToken("bad")).Code)
	assert.Equal(t, http.StatusUnauthorized, serve(t, deferred, "", installprofile.BearerToken(token)).Code)

	// ConfigMap absent (cluster older than this feature): not deferred.
	legacy := New(tokenFn, func() (map[string]string, error) {
		return nil, errors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "x")
	})
	rr = serve(t, legacy, "n1", installprofile.BearerToken(token))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"deferLonghornInstall":false}`, rr.Body.String())
}

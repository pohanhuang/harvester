package console

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harvester/harvester/pkg/installer/config"
	"github.com/harvester/harvester/pkg/util/installprofile"
)

func TestFetchInstallProfile(t *testing.T) {
	const token = "secret"
	signed := func(hashToken string) *httptest.Server {
		return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body := []byte(`{"deferLonghornInstall":true}`)
			w.Header().Set(installprofile.HashHeader, installprofile.Sign(hashToken, r.Header.Get(installprofile.NonceHeader), body))
			_, _ = w.Write(body)
		}))
	}

	ok := signed(token)
	defer ok.Close()
	p, err := fetchInstallProfile(ok.URL, token)
	assert.NoError(t, err)
	assert.True(t, p.DeferLonghornInstall)

	forged := signed("other")
	defer forged.Close()
	_, err = fetchInstallProfile(forged.URL, token)
	assert.ErrorContains(t, err, "signature mismatch")

	missingHeader := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"deferLonghornInstall":true}`))
	}))
	defer missingHeader.Close()
	_, err = fetchInstallProfile(missingHeader.URL, token)
	assert.ErrorContains(t, err, "signature mismatch")

	notFound := httptest.NewTLSServer(http.NotFoundHandler())
	defer notFound.Close()
	_, err = fetchInstallProfile(notFound.URL, token)
	assert.ErrorIs(t, err, errInstallProfileNotFound)
}

func TestApplyInstallProfile(t *testing.T) {
	cfg := &config.HarvesterConfig{}
	cfg.Install.DataDisk = "/dev/sdb"
	notice := applyInstallProfile(cfg, &installprofile.InstallProfile{DeferLonghornInstall: true})
	assert.True(t, cfg.Install.DeferLonghornInstall)
	assert.Empty(t, cfg.Install.DataDisk)
	assert.NotEmpty(t, notice)

	cfg.Install.DataDisk = "/dev/sdb"
	assert.Empty(t, applyInstallProfile(cfg, &installprofile.InstallProfile{}))
	assert.False(t, cfg.Install.DeferLonghornInstall)
	assert.Equal(t, "/dev/sdb", cfg.Install.DataDisk)
}

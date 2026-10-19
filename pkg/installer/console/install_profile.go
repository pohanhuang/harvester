package console

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/harvester/harvester/pkg/installer/config"
	"github.com/harvester/harvester/pkg/util/installprofile"
)

var errInstallProfileNotFound = errors.New("install profile not found")

// fetchInstallProfile gets the cluster InstallProfile from a joining node's
// view. The response is only trusted when its X-Cattle-Hash proves the server
// knows the join token. errInstallProfileNotFound means an older cluster.
func fetchInstallProfile(serverURL, token string) (*installprofile.InstallProfile, error) {
	client := http.Client{
		Timeout:   defaultHTTPTimeout,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // authenticity comes from the token HMAC
	}

	var profile *installprofile.InstallProfile
	var fatal error // not retried: retrying cannot fix a wrong token
	// After configuring the network, it needs a few seconds to be available.
	err := retryOnError(3, 2, func() error {
		nonceBytes := make([]byte, 16)
		if _, err := rand.Read(nonceBytes); err != nil {
			return err
		}
		nonce := hex.EncodeToString(nonceBytes)

		req, err := http.NewRequest(http.MethodGet, serverURL+installprofile.Path, nil)
		if err != nil {
			return err
		}
		req.Header.Set(installprofile.NonceHeader, nonce)
		req.Header.Set("Authorization", "Bearer "+installprofile.BearerToken(token))

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close() //nolint:errcheck
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		switch {
		case resp.StatusCode == http.StatusNotFound:
			return nil
		case resp.StatusCode != http.StatusOK:
			return fmt.Errorf("got %d status code from %s, body: %s", resp.StatusCode, req.URL, string(body))
		}
		if !installprofile.Verify(token, nonce, body, resp.Header.Get(installprofile.HashHeader)) {
			fatal = errors.New("install profile signature mismatch: wrong cluster token or tampered response")
			return nil
		}
		profile = &installprofile.InstallProfile{}
		return json.Unmarshal(body, profile)
	})
	if err != nil {
		return nil, err
	}
	if fatal != nil {
		return nil, fatal
	}
	if profile == nil {
		return nil, errInstallProfileNotFound
	}
	return profile, nil
}

// applyInstallProfile makes the joining node follow the cluster. It returns a
// notice for the user when it had to change something.
func applyInstallProfile(cfg *config.HarvesterConfig, p *installprofile.InstallProfile) string {
	cfg.Install.DeferLonghornInstall = p.DeferLonghornInstall
	if p.DeferLonghornInstall && cfg.Install.DataDisk != "" {
		cfg.Install.DataDisk = ""
		return "The cluster defers Longhorn installation; the selected data disk was cleared."
	}
	return ""
}

// syncInstallProfile fetches and applies the cluster InstallProfile for a
// joining node. An older cluster without the route is treated as not deferred.
func syncInstallProfile(c *Console, token string) error {
	profile, err := fetchInstallProfile(c.config.ServerURL, token)
	if errors.Is(err, errInstallProfileNotFound) {
		userInputData.InstallProfileNotice = "The cluster does not publish an install profile; assuming Longhorn is installed."
		profile = &installprofile.InstallProfile{}
	} else if err != nil {
		return err
	} else {
		userInputData.InstallProfileNotice = ""
	}
	if notice := applyInstallProfile(c.config, profile); notice != "" {
		userInputData.InstallProfileNotice = notice
	}
	return nil
}

package melu

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wolfi-dev/wolfictl/pkg/melange"
	"golang.org/x/time/rate"

	whttp "github.com/wolfi-dev/wolfictl/pkg/http"
)

type AnityaService struct {
	client   *whttp.RLHTTPClient
	packages map[string]*melange.Packages
}

func NewAnityaService(pkgs map[string]*melange.Packages) *AnityaService {
	var c *whttp.RLHTTPClient

	if t := os.Getenv("ANITYA_TOKEN"); t != "" {
		c = &whttp.RLHTTPClient{
			Ratelimiter: rate.NewLimiter(rate.Every(1*time.Second/2), 1),
			Client: &http.Client{
				Transport: &AuthTransport{
					Transport:     http.DefaultTransport,
					Authorization: fmt.Sprintf("token %s", t),
				},
			},
		}
	} else {
		c = &whttp.RLHTTPClient{
			Ratelimiter: rate.NewLimiter(rate.Every(5*time.Second), 1),
			Client:      http.DefaultClient,
		}
	}

	p := make(map[string]*melange.Packages)

	for pkgName, pkg := range pkgs {
		if pkg.Config.Update.ReleaseMonitor.Identifier < 1 {
			p[pkgName] = pkg
		}
	}

	return &AnityaService{client: c, packages: p}
}

func (o *AnityaService) GetLatestVersions() (map[string]NewVersionResults, error) {
	versions := make(map[string]NewVersionResults)

	for p := range o.packages {
		v, err := o.getLatestVersion(p)
		if err != nil {
			return nil, err
		}

		v = o.prepareVersion(p, v)

		versions[p] = NewVersionResults{Version: v}
	}

	return versions, nil
}

func (o *AnityaService) getLatestVersion(packageName string) (string, error) {	
	id := o.packages[packageName].Config.Update.ReleaseMonitor.Identifier

	url := fmt.Sprintf("https://release-monitoring.org/api/v2/versions/?project_id=%d", id)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %s", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("specified identifier doesn’t exist: %d", id)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("")
	}

	var version struct {
		Latest string `json:"latest_version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return "", fmt.Errorf("decoding version data: %w", err)
	}

	return version.Latest, nil
}

func (o *AnityaService) prepareVersion(packageName, v string) string {
	rm := o.packages[packageName].Config.Update.ReleaseMonitor

	if rm.StripPrefix != "" {
		v = strings.TrimPrefix(v, rm.StripPrefix)
	}

	if rm.StripSuffix != "" {
		v = strings.TrimSuffix(v, rm.StripSuffix)
	}

	return v
}

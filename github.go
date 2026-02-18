package melu

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/wolfi-dev/wolfictl/pkg/melange"
	"golang.org/x/time/rate"

	whttp "github.com/wolfi-dev/wolfictl/pkg/http"
)

// Git SHA should be 40 hexadecimal chars
var gitShaRe = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

var gitHubTmplFn = template.FuncMap{
	"dec": func(n int) int {
		return n - 1
	},
}

var gitHubTagsQueryTmpl = template.Must(template.New("").Funcs(gitHubTmplFn).Parse(`
query {
{{- $max := dec (len .RepoList) }}
{{- range  $index, $r := .RepoList }}
	r{{$r.PackageHash}}: repository(owner: "{{$r.Owner}}", name: "{{$r.Name}}") {
		refs(refPrefix: "refs/tags/", query: "{{$r.Filter}}", orderBy: {field: TAG_COMMIT_DATE, direction: DESC}, first: 1) {
			nodes {
				name
				target {
					commitUrl
				}
			}
		}
	}{{ if lt $index $max }},{{ end }}
{{- end }}
}
`)).Option("missingkey=error")

var gitHubReleasesQueryTmpl = template.Must(template.New("").Funcs(gitHubTmplFn).Parse(`
query {
{{- $max := dec (len .RepoList) }}
{{- range $index, $r := .RepoList }}
	r{{$r.PackageHash}}: repository(owner: "{{$r.Owner}}", name: "{{$r.Name}}") {
		latestRelease {
			tagName
			tagCommit {
				commitUrl
			}
		}
	}{{ if lt $index $max }},{{ end }}
{{- end }}
}
`)).Option("missingkey=error")

type gitHubReleases struct {
	Data map[string]struct {
		LatestRelease struct {
			TagName   string `json:"tagName"`
			TagCommit struct {
				CommitURL string `json:"commitUrl"`
			} `json:"tagCommit"`
		} `json:"latestRelease"`
	} `json:"data"`
}

type gitHubTags struct {
	Data map[string]struct {
		Refs struct {
			Nodes []struct {
				Name   string `json:"name"`
				Target struct {
					CommitURL string `json:"commitUrl"`
				} `json:"target"`
			} `json:"nodes"`
		} `json:"refs"`
	} `json:"data"`
}

type GitHubService struct {
	client       *whttp.RLHTTPClient
	packages     map[string]*melange.Packages
	packagesMap  map[string]string
	repoReleases []RepoInfo
	repoTags     []RepoInfo
}

type RepoInfo struct {
	Owner       string
	Name        string
	Filter      string
	PackageHash string
}

func NewGitHubService(pkgs map[string]*melange.Packages) (*GitHubService, error) {
	t := os.Getenv("GITHUB_TOKEN")
	if t == "" {
		return nil, fmt.Errorf("no GITHUB_TOKEN environment variable found, required by GitHub GraphQL API")
	}

	s := &GitHubService{
		client: &whttp.RLHTTPClient{
			Ratelimiter: rate.NewLimiter(rate.Every(5*time.Second), 1),
			Client: &http.Client{
				Transport: &AuthTransport{
					Transport:     http.DefaultTransport,
					Authorization: fmt.Sprintf("bearer %s", t),
				},
			},
		},
		packages:    make(map[string]*melange.Packages),
		packagesMap: make(map[string]string),
	}

	for pkgName, pkg := range pkgs {
		if m := pkg.Config.Update.GitHubMonitor; m != nil {
			r, err := s.getRepoInfo(pkg)
			if err != nil {
				continue
			}

			s.packages[pkgName] = pkg
			s.packagesMap[r.PackageHash] = pkgName

			if m.UseTags {
				s.repoTags = append(s.repoTags, r)
			} else {
				s.repoReleases = append(s.repoReleases, r)
			}
		}
	}

	return s, nil
}

func (o *GitHubService) GetLatestVersions() (map[string]NewVersionResults, error) {
	versions, err := o.getReleasesVersions()
	if err != nil {
		return nil, err
	}

	tagsVersions, err := o.getTagsVersions()
	if err != nil {
		return nil, err
	}

	maps.Copy(versions, tagsVersions)

	return versions, nil
}

func (o *GitHubService) getReleasesVersions() (map[string]NewVersionResults, error) {
	results := make(map[string]NewVersionResults)
	batchSize := 15

	r := o.repoReleases

	for i := 0; i < len(r); i += batchSize {
		end := min(i+batchSize, len(o.repoReleases))

		q, err := tmpl(gitHubReleasesQueryTmpl, map[string]any{"RepoList": r[i:end]})
		if err != nil {
			return results, fmt.Errorf("template graphql query: %w", err)
		}

		repos := gitHubReleases{}
		if err := o.doGraphQLQuery(q, &repos); err != nil {
			return results, err
		}

		for packageHash, releases := range repos.Data {
			packageHash = strings.TrimPrefix(packageHash, "r")
			packageName := o.packagesMap[packageHash]
			release := releases.LatestRelease

			version := o.prepareVersion(packageName, release.TagName)

			commit, err := getCommit(release.TagCommit.CommitURL)
			if err != nil {
				return results, fmt.Errorf("failed to get commit sha from commit URL %s: %w", release.TagCommit.CommitURL, err)
			}

			results[packageName] = NewVersionResults{
				Version: version,
				Commit:  commit,
			}
		}
	}

	return results, nil
}

func (o *GitHubService) getTagsVersions() (map[string]NewVersionResults, error) {
	results := make(map[string]NewVersionResults)
	batchSize := 15

	for i := 0; i < len(o.repoTags); i += batchSize {
		end := min(i+batchSize, len(o.repoTags))

		q, err := tmpl(gitHubTagsQueryTmpl, map[string]any{"RepoList": o.repoTags[i:end]})
		if err != nil {
			return results, fmt.Errorf("template graphql query: %w", err)
		}

		repos := gitHubTags{}
		if err := o.doGraphQLQuery(q, &repos); err != nil {
			return results, err
		}

		for packageHash, tags := range repos.Data {
			packageHash = strings.TrimPrefix(packageHash, "r")
			packageName := o.packagesMap[packageHash]
			tag := tags.Refs.Nodes[0]

			version := o.prepareVersion(packageName, tag.Name)

			commit, err := getCommit(tag.Target.CommitURL)
			if err != nil {
				return nil, fmt.Errorf("failed to get commit sha from commit URL %s: %w", tag.Target.CommitURL, err)
			}

			results[packageName] = NewVersionResults{
				Version: version,
				Commit:  commit,
			}
		}
	}

	return results, nil
}

func (o *GitHubService) getRepoInfo(p *melange.Packages) (RepoInfo, error) {
	m := p.Config.Update.GitHubMonitor

	parts := strings.Split(m.Identifier, "/")
	if len(parts) != 2 {
		return RepoInfo{}, fmt.Errorf("malformed repo identifier should be in the form owner/repo, got %s", m.Identifier)
	}

	var filter string

	switch {
	case m.TagFilterPrefix != "":
		filter = m.TagFilterPrefix
	case m.TagFilterContains != "":
		filter = m.TagFilterContains
	default:
		filter = m.TagFilter
	}

	return RepoInfo{
		Owner:       parts[0],
		Name:        parts[1],
		Filter:      filter,
		PackageHash: h1(p.Config.Package.Name),
	}, nil
}

func (o *GitHubService) doGraphQLQuery(query string, response any) error {
	payload, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.github.com/graphql", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non ok http response for github graphql code: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(response)
}

func (o *GitHubService) prepareVersion(packageName, v string) string {
	ghm := o.packages[packageName].Config.Update.GitHubMonitor

	if ghm.StripPrefix != "" {
		v = strings.TrimPrefix(v, ghm.StripPrefix)
	}

	if ghm.StripSuffix != "" {
		v = strings.TrimSuffix(v, ghm.StripSuffix)
	}

	return v
}

func tmpl(t *template.Template, d any) (string, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func getCommit(url string) (string, error) {
	parts := strings.Split(url, "/")
	sha := parts[len(parts)-1]
	if !gitShaRe.MatchString(sha) {
		return "", fmt.Errorf("%s is not a sha", sha)
	}
	return sha, nil
}

func h1(s string) string {
	sum := sha1.Sum([]byte(s))
	return fmt.Sprintf("%x", sum)
}

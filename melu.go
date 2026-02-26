package melu

import (
	"context"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chainguard.dev/melange/pkg/renovate"
	"chainguard.dev/melange/pkg/renovate/bump"
	"github.com/wolfi-dev/wolfictl/pkg/melange"

	git "github.com/go-git/go-git/v5"
	gitobject "github.com/go-git/go-git/v5/plumbing/object"
	wgit "github.com/wolfi-dev/wolfictl/pkg/git"
)

const (
	defaultGitAuthorName = "melu"
	defaultGitAuthorEmail = "melu@example.com"
)

type Options struct {
	RepoURI    string
	RepoBranch string

	PackagePath      string
	PackageNames     []string
	PackageConfigs   map[string]*melange.Packages
	PackagesToUpdate map[string]NewVersionResults

	GitQuery    bool
	GitHubQuery bool
	AnityaQuery bool

	Push bool
	Clean bool

	Logger *log.Logger
}

type AuthTransport struct {
	Transport     http.RoundTripper
	Authorization string
}

func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Authorization != "" {
		req.Header.Set("Authorization", t.Authorization)
	}
	return t.Transport.RoundTrip(req)
}

type NewVersionResults struct {
	Version string
	Commit  string
}

func New() Options {
	return Options{
		Logger: log.New(log.Writer(), "", log.LstdFlags|log.Lmsgprefix),
	}
}

func (o *Options) Update(ctx context.Context) error {
	// Clone the melange config git repo into a temp folder so we can work with it
	tempDir, err := os.MkdirTemp("", "melu")
	if err != nil {
		return fmt.Errorf("unable to create temp directory for git clone: %w", err)
	}

	cwd, _ := os.Getwd()

	if err = os.Chdir(tempDir); err != nil {
		return fmt.Errorf("unable to chdir into temp directory: %w", err)
	}

	defer func() {
		os.Chdir(cwd)
		if o.Clean {
			os.Remove(tempDir)
		}
		
	}()
	
	auth, err := wgit.GetGitAuth(o.RepoURI)
	if err != nil {
		return fmt.Errorf("unable to get git auth: %w", err)
	}

	repo, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:   o.RepoURI,
		Auth:  auth,
		Depth: 1,
	})
	if err != nil {
		return fmt.Errorf("unable to clone repo %q to temp directory: %w", o.RepoURI, err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("git worktree: %s", err)
	}

	// Parse configurations and look for new versions
	latestVersions, err := o.GetLatestVersions(ctx, tempDir)
	if err != nil {
		return fmt.Errorf("get latest versions: %w", err)
	}

	var gitAuthorName, gitAuthorEmail string

	if gitAuthorName = os.Getenv("GIT_AUTHOR_NAME"); gitAuthorName == "" {
		gitAuthorName = defaultGitAuthorName
	}

	if gitAuthorEmail = os.Getenv("GIT_AUTHOR_EMAIL"); gitAuthorEmail == "" {
		gitAuthorEmail = defaultGitAuthorEmail
	}

	var updatedPackages []string

	for packageName, latest := range latestVersions {
		pkg := o.PackageConfigs[packageName]

		// Compare current version with the latest
		if pkg.Config.Package.Version == latest.Version {
			continue
		}

		// Bump package version
		renovator, err := renovate.New(renovate.WithConfig(filepath.Join(pkg.Dir, pkg.Filename)))
		if err != nil {
			return err
		}

		if err := renovator.Renovate(ctx,
			bump.New(ctx,
				bump.WithTargetVersion(latest.Version),
				bump.WithExpectedCommit(latest.Commit),
			),
		); err != nil {
			o.Logger.Printf("renovate %s: %s\n", packageName, err)
			continue
		}

		// Commit bumped package
		if _, err = wt.Add(filepath.Join(o.PackagePath, pkg.Filename)); err != nil {
			return fmt.Errorf("git add: %w", err)
		}
		if _, err = wt.Commit(fmt.Sprintf("Update %s package version to %s", packageName, latest.Version),
			&git.CommitOptions{
				Author: &gitobject.Signature{
					Name:  gitAuthorName,
					Email: gitAuthorEmail,
					When:  time.Now(),
				},
			}); err != nil {
				return fmt.Errorf("git commit: %w", err)
			}
		
		updatedPackages = append(updatedPackages, packageName)
	}

	if len(updatedPackages) < 1 {
		return nil
	}

	if !o.Push {
		return nil
	}

	if gho := os.Getenv("GITHUB_OUTPUT"); gho != "" {
		o.Logger.Printf("writing updated packages to %s:\n", gho)
		
		f, err := os.OpenFile(gho, os.O_WRONLY, 0644);
		if err != nil {
			return err
		}

		_, err = fmt.Fprintf(f, "packages=%s\n", strings.Join(updatedPackages, ","))
		if err != nil {
			return err

		}
	}

	return repo.Push(&git.PushOptions{Auth: auth})
}

func (o *Options) GetLatestVersions(ctx context.Context, dir string) (map[string]NewVersionResults, error) {
	var err error
	latestVersions := make(map[string]NewVersionResults)

	o.PackageConfigs = make(map[string]*melange.Packages)

	pkgs, err := melange.ReadPackageConfigs(ctx, o.PackageNames, filepath.Join(dir, o.PackagePath))
	if err != nil {
		return nil, fmt.Errorf("unable to get package configs: %w", err)
	}

	for pkgName, pkg := range pkgs {
		if pkg.Config.Update.Enabled {
			o.PackageConfigs[pkgName] = pkg
		}
	}

	if len(o.PackageConfigs) < 1 {
		o.Logger.Printf("no packages with updates enabled found\n")
		return nil, nil
	}

	if o.GitQuery {
		s := NewGitService(o.PackageConfigs)
		v, err := s.GetLatestVersions()
		if err != nil {
			return latestVersions, fmt.Errorf("failed getting git repositories versions: %w", err)
		}
		maps.Copy(latestVersions, v)
	}

	if o.GitHubQuery {
		s, err := NewGitHubService(o.PackageConfigs)
		if err != nil {
			return latestVersions, fmt.Errorf("failed to initialise github service: %w", err)
		}
		v, err := s.GetLatestVersions()
		if err != nil {
			return latestVersions, fmt.Errorf("failed getting github releases: %w", err)
		}
		maps.Copy(latestVersions, v)
	}

	if o.AnityaQuery {
		s := NewAnityaService(o.PackageConfigs)
		v, err := s.GetLatestVersions()
		if err != nil {
			return latestVersions, fmt.Errorf("failed getting release monitor versions: %w", err)
		}
		maps.Copy(latestVersions, v)
	}

	return latestVersions, nil
}

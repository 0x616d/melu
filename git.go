package melu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0x616d/melu/internal/melange"

	git "github.com/go-git/go-git/v5"
	gitplumbing "github.com/go-git/go-git/v5/plumbing"
	gitobject "github.com/go-git/go-git/v5/plumbing/object"
)

type GitService struct {
	packages map[string]*melange.Package
}

func NewGitService(pkgs map[string]*melange.Package) *GitService {
	p := make(map[string]*melange.Package)

	for pkgName, pkg := range pkgs {
		if pkg.Config.Update.GitMonitor == nil {
			continue
		}

		if _, err := findGitCheckoutOptions(pkg); err != nil {
			continue
		}

		p[pkgName] = pkg
	}

	return &GitService{packages: p}
}

func (o *GitService) GetLatestVersions() (map[string]NewVersionResults, error) {
	versions := make(map[string]NewVersionResults)

	for pkgName := range o.packages {
		v, err := o.getLatestVersion(o.packages[pkgName])
		if err != nil {
			return versions, err
		}

		versions[pkgName] = v
	}

	return versions, nil
}

func (o *GitService) getLatestVersion(pkg *melange.Package) (NewVersionResults, error) {
	cloneDir := filepath.Join(os.TempDir(), "melu"+pkg.Config.Package.Name)

	checkoutOpts, _ := findGitCheckoutOptions(pkg)

	repo, _ := checkoutOpts["repository"]

	r, err := git.PlainClone(cloneDir, false, &git.CloneOptions{URL: repo})
	if err != nil {
		return NewVersionResults{}, err
	}
	defer os.RemoveAll(cloneDir)

	if _, ok := checkoutOpts["tag"]; ok {
		gm := pkg.Config.Update.GitMonitor

		tag, err := o.getLatestTagFromRepo(r, gm.TagFilterPrefix, gm.TagFilterContains)
		if err != nil {
			return NewVersionResults{}, err
		}

		return NewVersionResults{
			Version: o.prepareVersion(pkg.Config.Package.Name, tag.Name),
			Commit:  tag.Hash.String(),
		}, nil
	}

	var ref *gitplumbing.Reference

	if branch, ok := checkoutOpts["branch"]; ok {
		ref, err = r.Reference(gitplumbing.ReferenceName("refs/heads/"+branch), false)
	} else {
		ref, err = r.Head()
	}

	if err != nil {
		return NewVersionResults{}, err
	}

	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		return NewVersionResults{}, err
	}

	version := strings.Split(pkg.Config.Package.Version, "_git")[0]
	if version == "" {
		version = "0"
	}

	return NewVersionResults{
		Version: fmt.Sprintf("%s_git%s", version, commit.Committer.When.Format("20060102")),
		Commit:  commit.Hash.String(),
	}, nil
}

func (o *GitService) getLatestTagFromRepo(r *git.Repository, filterPrefix, filterContains string) (*gitobject.Tag, error) {
	tags, err := r.TagObjects()
	if err != nil {
		return nil, err
	}

	var latestTag *gitobject.Tag
	var latestTagCommit *gitobject.Commit

	err = tags.ForEach(func(tag *gitobject.Tag) error {
		commit, err := tag.Commit()
		if err != nil {
			return err
		}

		if filterPrefix != "" && !strings.HasPrefix(tag.Name, filterPrefix) {
			return nil
		}

		if filterContains != "" && !strings.Contains(tag.Name, filterContains) {
			return nil
		}

		if latestTagCommit == nil {
			latestTag = tag
			latestTagCommit = commit
			return nil
		}

		if commit.Committer.When.After(latestTagCommit.Author.When) {
			latestTag = tag
			latestTagCommit = commit
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return latestTag, nil
}

func (o *GitService) prepareVersion(packageName, v string) string {
	gm := o.packages[packageName].Config.Update.GitMonitor

	if gm.StripPrefix != "" {
		v = strings.TrimPrefix(v, gm.StripPrefix)
	}

	if gm.StripSuffix != "" {
		v = strings.TrimSuffix(v, gm.StripSuffix)
	}

	return v
}

func findGitCheckoutOptions(pkg *melange.Package) (map[string]string, error) {
	if len(pkg.Config.Pipeline) == 0 {
		return nil, fmt.Errorf("no pipelines found in %s package", pkg.Config.Package.Name)
	}

	p := pkg.Config.Pipeline[0]

	if p.Uses != "git-checkout" {
		return nil, errors.New("no git-checkout step in pipeline")
	}

	if _, ok := p.With["repository"]; !ok {
		return nil, errors.New("no repository defined in git-checkout")
	}

	return p.With, nil
}

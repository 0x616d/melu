package melu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"chainguard.dev/melange/pkg/renovate"
	"github.com/dprotaso/go-yit"
	"github.com/wolfi-dev/wolfictl/pkg/melange"

	git "github.com/go-git/go-git/v5"
	gitplumbing "github.com/go-git/go-git/v5/plumbing"
)

type GitService struct {
	packages map[string]*melange.Packages
	repos    map[string]*gitCheckoutOpts
}

func NewGitService(pkgs map[string]*melange.Packages) *GitService {
	p := make(map[string]*melange.Packages)
	r := make(map[string]*gitCheckoutOpts)

	for pkgName, pkg := range pkgs {
		if pkg.Config.Update.GitMonitor == nil {
			continue
		}

		// packages must contain _git in the version to be updated by the Git updater
		if !strings.Contains(pkg.Config.Package.Version, "_git") {
			continue
		}

		checkoutOpts, err := getGitChechoutOpts(pkg)
		if err != nil {
			continue
		}

		r[pkgName] = checkoutOpts
		p[pkgName] = pkg
	}

	return &GitService{packages: p, repos: r}
}

func (o *GitService) GetLatestVersions() (map[string]NewVersionResults, error) {
	versions := make(map[string]NewVersionResults)

	for pkgName := range o.packages {
		v, err := o.getLatestVersion(o.packages[pkgName], o.repos[pkgName])
		if err != nil {
			return versions, err
		}

		versions[pkgName] = v
	}

	return versions, nil
}

func (o *GitService) getLatestVersion(pkg *melange.Packages, checkoutOpts *gitCheckoutOpts) (NewVersionResults, error) {
	cloneDir := filepath.Join(os.TempDir(), "melu"+pkg.Config.Package.Name)

	r, err := git.PlainClone(cloneDir, false, &git.CloneOptions{URL: checkoutOpts.Repository, Depth: checkoutOpts.Depth})
	if err != nil {
		return NewVersionResults{}, err
	}

	defer os.RemoveAll(cloneDir)

	var refName gitplumbing.ReferenceName

	switch {
	case checkoutOpts.Tag != "":
		refName = gitplumbing.ReferenceName("refs/tags/" + checkoutOpts.Tag)
	case checkoutOpts.Branch != "":
		refName = gitplumbing.ReferenceName("refs/heads/" + checkoutOpts.Branch)
	}

	var ref *gitplumbing.Reference

	if refName != "" {
		ref, err = r.Reference(refName, false)
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

	parts := strings.Split(pkg.Config.Package.Version, "_git")

	return NewVersionResults{
		Version: fmt.Sprintf("%s_git%s", parts[0], commit.Author.When.Format("20060102")),
		Commit:  commit.Hash.String(),
	}, nil
}

type gitCheckoutOpts struct {
	Repository string
	Branch     string
	Tag        string
	Depth      int
}

func getGitChechoutOpts(pkg *melange.Packages) (*gitCheckoutOpts, error) {
	cfg := pkg.Config

	pipelineNode, err := renovate.NodeFromMapping(cfg.Root().Content[0], "pipeline")
	if err != nil {
		return nil, err
	}

	it := yit.FromNode(pipelineNode).
		RecurseNodes().
		Filter(yit.WithMapValue("git-checkout"))

	gitCheckoutNode, ok := it()
	if !ok {
		return nil, errors.New("no git-checkout step in pipeline")
	}

	withNode, err := renovate.NodeFromMapping(gitCheckoutNode, "with")
	if err != nil {
		return nil, errors.New("git-checkout without options")
	}

	repository, err := renovate.NodeFromMapping(withNode, "repository")
	if err != nil {
		return nil, errors.New("no repository defined in git-checkout")
	}

	checkout := &gitCheckoutOpts{Repository: repository.Value}

	if branch, err := renovate.NodeFromMapping(withNode, "branch"); err == nil {
		checkout.Branch = branch.Value
	}

	if tag, err := renovate.NodeFromMapping(withNode, "tag"); err == nil {
		checkout.Tag = tag.Value
	}

	if depth, err := renovate.NodeFromMapping(withNode, "depth"); err == nil {
		if d, err := strconv.Atoi(depth.Value); err == nil {
			checkout.Depth = d
		} else {
			checkout.Depth = -1
		}
	} else {
		checkout.Depth = -1
	}

	return checkout, nil
}

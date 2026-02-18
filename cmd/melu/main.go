package main

import (
	"context"
	"flag"
	"net/url"

	"github.com/0x616d/melu"
)

func main() {
	u := melu.New()

	flag.BoolVar(&u.GitQuery, "query-git", false, "query git repositories for latest releases")
	flag.BoolVar(&u.GitHubQuery, "query-github", false, "query https://api.github.com/graphql API for latest releases")
	flag.BoolVar(&u.AnityaQuery, "query-anitya", false, "query https://release-monitoring.org API for latest releases")
	flag.BoolVar(&u.Push, "push", false, "push changes")
	flag.BoolVar(&u.Clean, "clean", true, "delete temporary cloned repository")
	flag.StringVar(&u.RepoBranch, "branch", "master", "repository branch")
	flag.StringVar(&u.PackagePath, "path", "", "path in the git repo containing the melange yaml files")

	flag.Parse()

	if flag.NArg() < 1 {
		u.Logger.Fatalf("usage: melu [flag]... repository\n")
	}

	repoURI := flag.Arg(0)

	if _, err := url.ParseRequestURI(repoURI); err != nil {
		u.Logger.Fatalf("parse repository URI %s: %s\n", repoURI, err)
	}

	u.RepoURI = repoURI

	if err := u.Update(context.Background()); err != nil {
		u.Logger.Fatalln(err)
	}
}

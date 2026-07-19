package main

import (
	"encoding/csv"
	"flag"
	"log"
	"maps"
	"os"

	"github.com/0x616d/melu"
	"github.com/0x616d/melu/internal/melange"
)

func main() {
	var (
		queryGit    bool
		queryGitHub bool
		queryAnitya bool
	)

	flag.BoolVar(&queryGit, "query-git", false, "query git repositories for latest releases")
	flag.BoolVar(&queryGitHub, "query-github", false, "query https://api.github.com/graphql API for latest releases")
	flag.BoolVar(&queryAnitya, "query-anitya", false, "query https://release-monitoring.org API for latest releases")
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	pkgs, err := melange.ReadPackageConfigs(flag.Args())
	if err != nil {
		log.Fatalf("read packages: %s\n", err)
	}

	latestVersions := make(map[string]melu.NewVersionResults)

	if !queryGit && !queryGitHub && !queryAnitya {
		log.Fatalf("no services from where to fetch latests versions\n")
	}

	if queryGit {
		s := melu.NewGitService(pkgs)
		v, err := s.GetLatestVersions()
		if err != nil {
			log.Fatalf("get git repositories versions: %s\n", err)
		}
		maps.Copy(latestVersions, v)
	}

	if queryGitHub {
		s, err := melu.NewGitHubService(pkgs)
		if err != nil {
			log.Fatalf("new github service: %s\n", err)
		}
		v, err := s.GetLatestVersions()
		if err != nil {
			log.Fatalf("get latest github tag/releases: %s\n", err)
		}
		maps.Copy(latestVersions, v)
	}

	if queryAnitya {
		s := melu.NewAnityaService(pkgs)
		v, err := s.GetLatestVersions()
		if err != nil {
			log.Fatalf("get release monitor versions: %s\n", err)
		}
		maps.Copy(latestVersions, v)
	}

	w := csv.NewWriter(os.Stdout)

	for packageName, latest := range latestVersions {
		pkg := pkgs[packageName]

		if err := w.Write([]string{
			pkg.File,
			pkg.Config.Package.Name,
			pkg.Config.Package.Version,
			latest.Version,
			latest.Commit,
		}); err != nil {
			log.Fatalf("csv write: %s\n", err)
		}
	}

	w.Flush()
}

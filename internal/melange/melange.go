package melange

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0x616d/melu/internal/melange/config"

	"go.yaml.in/yaml/v4"
)

type Package struct {
	File   string
	Config config.Configuration
}

func ReadPackageConfigs(inputs []string) (map[string]*Package, error) {
	files, err := findYAMLFiles(inputs)
	if err != nil {
		return nil, err
	}

	p := make(map[string]*Package)

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}

		cfg := config.Configuration{}

		if err := yaml.Unmarshal(data, &cfg); err != nil {
			continue
		}

		if cfg.Package.Name == "" || cfg.Package.Version == "" {
			continue
		}

		fn := filepath.Base(f)
		dir := filepath.Dir(f)
		name := cfg.Package.Name

		if name != strings.TrimSuffix(fn, filepath.Ext(fn)) {
			return nil, fmt.Errorf("package name does not match file name in %q: %s", f, name)
		}

		if _, exists := p[name]; exists {
			return nil, fmt.Errorf("duplicate package %s found in %q and %q", name, filepath.Dir(p[name].File), dir)
		}

		p[name] = &Package{
			File:   f,
			Config: cfg,
		}
	}

	return p, nil
}

func findYAMLFiles(inputs []string) ([]string, error) {
	var results []string

	for _, in := range inputs {
		if in == "" {
			continue
		}

		absPath, err := filepath.Abs(in)
		if err != nil {
			return nil, fmt.Errorf("absolute path %s: %w", in, err)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", absPath, err)
		}

		if info.IsDir() {
			err := filepath.WalkDir(absPath, func(path string, d os.DirEntry, _ error) error {
				if d.Type().IsDir() && path != absPath {
					return filepath.SkipDir
				}
				if d.Type().IsRegular() && extIsYAML(path) {
					results = append(results, path)
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("walk %s: %w", absPath, err)
			}
			continue
		}

		if info.Mode().IsRegular() && extIsYAML(absPath) {
			results = append(results, absPath)
		}
	}

	sort.Strings(results)

	return results, nil
}

func extIsYAML(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}

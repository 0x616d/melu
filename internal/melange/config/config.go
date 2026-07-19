package config

// Configuration is the root melange configuration.
type Configuration struct {
	Package  Package    `yaml:"package"`
	Pipeline []Pipeline `yaml:"pipeline,omitempty"`
	Update   Update     `yaml:"update,omitempty"`
}

type Package struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Pipeline struct {
	Uses string            `yaml:"uses,omitempty"`
	With map[string]string `yaml:"with,omitempty"`
}

// Update provides information used to describe how to keep the package up to date
type Update struct {
	// Toggle if updates should occur
	Enabled bool `yaml:"enabled"`
	// The configuration block for updates tracked via release-monitoring.org
	ReleaseMonitor *ReleaseMonitor `yaml:"release-monitor,omitempty"`
	// The configuration block for updates tracked via the Github API
	GitHubMonitor *GitHubMonitor `yaml:"github,omitempty"`
	// The configuration block for updates tracked via Git
	GitMonitor *GitMonitor `yaml:"git,omitempty"`
	// The configuration block for updates tracked via OCI image tags
	OCIMonitor *OCIMonitor `yaml:"oci,omitempty"`
	// The configuration block for transforming the `package.version` into an APK version
	VersionTransform []VersionTransform `yaml:"version-transform,omitempty"`
}

// ReleaseMonitor indicates using the API for https://release-monitoring.org/
type ReleaseMonitor struct {
	// Required: ID number for release monitor
	Identifier int `json:"identifier" yaml:"identifier"`
	// If the version in release monitor contains a prefix which should be ignored
	StripPrefix string `json:"strip-prefix,omitempty" yaml:"strip-prefix,omitempty"`
	// If the version in release monitor contains a suffix which should be ignored
	StripSuffix string `json:"strip-suffix,omitempty" yaml:"strip-suffix,omitempty"`
	// Filter to apply when searching version on a Release Monitoring
	VersionFilterContains string `json:"version-filter-contains,omitempty" yaml:"version-filter-contains,omitempty"`
	// Filter to apply when searching version Release Monitoring
	VersionFilterPrefix string `json:"version-filter-prefix,omitempty" yaml:"version-filter-prefix,omitempty"`
}

// GitHubMonitor indicates using the GitHub API
type GitHubMonitor struct {
	// Org/repo for GitHub
	Identifier string `json:"identifier" yaml:"identifier"`
	// If the version in GitHub contains a prefix which should be ignored
	StripPrefix string `json:"strip-prefix,omitempty" yaml:"strip-prefix,omitempty"`
	// If the version in GitHub contains a suffix which should be ignored
	StripSuffix string `json:"strip-suffix,omitempty" yaml:"strip-suffix,omitempty"`
	// Filter to apply when searching tags on a GitHub repository
	//
	// Deprecated: Use TagFilterPrefix instead
	TagFilter string `json:"tag-filter,omitempty" yaml:"tag-filter,omitempty"`
	// Prefix filter to apply when searching tags on a GitHub repository
	TagFilterPrefix string `json:"tag-filter-prefix,omitempty" yaml:"tag-filter-prefix,omitempty"`
	// Filter to apply when searching tags on a GitHub repository
	TagFilterContains string `json:"tag-filter-contains,omitempty" yaml:"tag-filter-contains,omitempty"`
	// Override the default of using a GitHub release to identify related tag to
	// fetch.  Not all projects use GitHub releases but just use tags
	UseTags bool `json:"use-tag,omitempty" yaml:"use-tag,omitempty"`
}

// GitMonitor indicates using Git
type GitMonitor struct {
	// StripPrefix is the prefix to strip from the version
	StripPrefix string `json:"strip-prefix,omitempty" yaml:"strip-prefix,omitempty"`
	// If the version contains a suffix which should be ignored
	StripSuffix string `json:"strip-suffix,omitempty" yaml:"strip-suffix,omitempty"`
	// Prefix filter to apply when searching tags on a Git repository
	TagFilterPrefix string `json:"tag-filter-prefix,omitempty" yaml:"tag-filter-prefix,omitempty"`
	// Filter to apply when searching tags on a Git repository
	TagFilterContains string `json:"tag-filter-contains,omitempty" yaml:"tag-filter-contains,omitempty"`
}

// OCIMonitor indicates using OCI image tags
type OCIMonitor struct {
	// Required: OCI image reference (e.g. cgr.dev/chainguard/node)
	Identifier string `json:"identifier" yaml:"identifier"`
	// If the version in the tag contains a prefix which should be ignored
	StripPrefix string `json:"strip-prefix,omitempty" yaml:"strip-prefix,omitempty"`
	// If the version in the tag contains a suffix which should be ignored
	StripSuffix string `json:"strip-suffix,omitempty" yaml:"strip-suffix,omitempty"`
	// Prefix filter to apply when searching tags
	TagFilterPrefix string `json:"tag-filter-prefix,omitempty" yaml:"tag-filter-prefix,omitempty"`
	// Substring filter to apply when searching tags
	TagFilterContains string `json:"tag-filter-contains,omitempty" yaml:"tag-filter-contains,omitempty"`
}

// VersionTransform allows mapping the package version to an APK version
type VersionTransform struct {
	// Required: The regular expression to match against the `package.version` variable
	Match string `json:"match" yaml:"match"`
	// Required: The repl to replace on all `match` matches
	Replace string `json:"replace" yaml:"replace"`
}

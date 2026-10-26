package schema

// Blocklist is the YAML shape of config/companies/blocklist.yaml.
// Exact and Anonymous block creation; Review only sets NeedsReview.
type Blocklist struct {
	Exact     []string `yaml:"exact"`
	Anonymous []string `yaml:"anonymous"`
	Review    []string `yaml:"review"`
}

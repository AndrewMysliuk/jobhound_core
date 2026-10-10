package schema

// Profile is one search definition loaded from YAML.
type Profile struct {
	ID             string
	Name           string
	Domain         Domain
	Sources        []string
	Queries        []string
	WellfoundRoles []string
	ExcludeTitle   []string
	ExcludeText    []string
	Penalties      []string
}

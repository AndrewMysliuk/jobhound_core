package config

import (
	"os"
	"strings"
)

// EnvProfilesDir is the directory of search-profile YAML files.
const EnvProfilesDir = "JOBHOUND_PROFILES_DIR"

// DefaultProfilesDir is used when JOBHOUND_PROFILES_DIR is unset or blank, relative to the process working directory.
const DefaultProfilesDir = "config/profiles"

func loadProfilesDirFromEnv() string {
	dir := strings.TrimSpace(os.Getenv(EnvProfilesDir))
	if dir == "" {
		return DefaultProfilesDir
	}
	return dir
}

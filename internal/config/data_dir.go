package config

import "os"

// EnvDataDir is the directory that contains shared data files (e.g. countries.json).
const EnvDataDir = "JOBHOUND_DATA_DIR"

func loadDataDirFromEnv() string {
	return os.Getenv(EnvDataDir)
}

package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	DBPath     string
	ListenAddr string
	LogDir     string
	DataDir    string
}

func Default() Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".keysat")
	return Config{
		DBPath:     filepath.Join(dataDir, "keysat.db"),
		ListenAddr: "127.0.0.1:7890",
		LogDir:     filepath.Join(dataDir, "logs"),
		DataDir:    dataDir,
	}
}

func (c Config) EnsureDirs() error {
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return err
	}
	return os.MkdirAll(c.LogDir, 0o755)
}

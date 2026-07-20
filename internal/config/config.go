package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Home        string
	SDKDir      string
	DefaultLink string
	LocalGoDir  string
}

func Default() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve home directory: %w", err)
	}

	return Config{
		Home:        home,
		SDKDir:      filepath.Join(home, "sdk"),
		DefaultLink: filepath.Join(home, "sdk", "go-current"),
		LocalGoDir:  filepath.Join(home, ".local", "go"),
	}, nil
}

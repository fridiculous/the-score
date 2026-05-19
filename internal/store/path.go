package store

import (
	"os"
	"path/filepath"
	"runtime"
)

const EnvDataDir = "SCORE_DATA_DIR"

func DefaultDataDir() (string, error) {
	if value := os.Getenv(EnvDataDir); value != "" {
		return value, nil
	}
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "the-score"), nil
	case "windows":
		if value := os.Getenv("LOCALAPPDATA"); value != "" {
			return filepath.Join(value, "the-score"), nil
		}
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "the-score"), nil
	default:
		if value := os.Getenv("XDG_DATA_HOME"); value != "" {
			return filepath.Join(value, "the-score"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "the-score"), nil
	}
}

func DefaultSQLitePath() (string, error) {
	dir, err := DefaultDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "score.db"), nil
}

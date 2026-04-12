package feeds

import (
	"encoding/json"
	"fmt"
	"news-cli/internal/models"
	"os"
	"path/filepath"
)

type filePayload struct {
	Links []models.FeedSource `json:"links"`
}

func Path() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not determine config directory: %w", err)
	}

	return filepath.Join(configDir, "recon", "links.json"), nil
}

func LoadData(defaultData []byte) ([]byte, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read feeds file: %w", readErr)
		}
		if err := Validate(data); err != nil {
			return nil, fmt.Errorf("invalid feeds file %s: %w", path, err)
		}
		return data, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to stat feeds file: %w", err)
	}

	if len(defaultData) == 0 {
		return []byte(`{"links":[]}`), nil
	}

	if err := Validate(defaultData); err != nil {
		return nil, fmt.Errorf("embedded feeds are invalid: %w", err)
	}

	if err := os.WriteFile(path, defaultData, 0644); err != nil {
		return nil, fmt.Errorf("failed to seed feeds file: %w", err)
	}

	return defaultData, nil
}

func Validate(data []byte) error {
	var payload filePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	return nil
}

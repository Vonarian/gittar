package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// NotificationSettings contains fine-tuned preferences for desktop alerts.
type NotificationSettings struct {
	Enabled         bool `json:"enabled"`
	PipelineSuccess bool `json:"pipelineSuccess"`
	PipelineFailed  bool `json:"pipelineFailed"`
	MRAssigned      bool `json:"mrAssigned"`
	MRReviewRequest bool `json:"mrReviewRequest"`
	TodoMention     bool `json:"todoMention"`
	TodoAssignment  bool `json:"todoAssignment"`
	TodoIssue       bool `json:"todoIssue"`
	TodoGeneric     bool `json:"todoGeneric"`
}

// Config holds the user settings for Gittar.
type Config struct {
	GitLabURL         string               `json:"gitlabUrl"`
	Token             string               `json:"token"`
	MonitoredGroups   []string             `json:"monitoredGroups"`
	MonitoredProjects []string             `json:"monitoredProjects"`
	PollIntervalSec   int                  `json:"pollIntervalSec"`
	Notifications     NotificationSettings `json:"notifications"`
}

// GetConfigDir returns the standard config directory for Gittar (~/.config/gittar).
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "gittar")
	return dir, nil
}

// LoadConfig reads the configuration file from disk. If the file does not exist, it returns a default config.
func LoadConfig() (*Config, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(dir, "config.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &Config{
			GitLabURL:         "https://gitlab.com",
			Token:             "",
			MonitoredGroups:   []string{},
			MonitoredProjects: []string{},
			PollIntervalSec:   30,
			Notifications: NotificationSettings{
				Enabled:         true,
				PipelineSuccess: true,
				PipelineFailed:  true,
				MRAssigned:      true,
				MRReviewRequest: true,
				TodoMention:     true,
				TodoAssignment:  true,
				TodoIssue:       true,
				TodoGeneric:     true,
			},
		}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var conf Config
	if err := json.Unmarshal(data, &conf); err != nil {
		return nil, err
	}

	if conf.GitLabURL == "" {
		conf.GitLabURL = "https://gitlab.com"
	}
	if conf.PollIntervalSec <= 0 {
		conf.PollIntervalSec = 30
	}

	return &conf, nil
}

// SaveConfig writes the configuration file to disk.
func SaveConfig(conf *Config) error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(dir, "config.json")
	data, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0600)
}

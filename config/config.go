// SPDX-License-Identifier: GPL-3.0-or-later
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Database string

	ImapHost string
	User     string
	Password string

	SpamassassinHost string

	DryRun bool

	MoveSpam      bool
	DeleteSpam    bool
	SpamFolder    string
	AppendReports bool
	ReportFolder  string

	CheckFolders []string

	SpamLearnFolders []string
	HamLearnFolders  []string
	DeleteLearned    bool

	Loglevel *string
}

func ReadConfig(filename string) (*Config, error) {
	config := &Config{
		Database:         "persistence.db",
		SpamassassinHost: "127.0.0.1:783",
		CheckFolders:     []string{"INBOX"},
		DryRun:           true,
	}

	_, err := toml.DecodeFile(filename, config)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	return config, nil
}

// SPDX-License-Identifier: GPL-3.0-or-later
package config

import (
	"errors"
	"fmt"
	"strings"

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
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	err = config.validate()
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) validate() error {
	if err := validateNonEmptyStringField(c.Database, "Database name must not be empty, set to a filename for the sqlite database"); err != nil {
		return err
	}

	if err := validateNonEmptyStringField(c.ImapHost, "ImapHost must not be empty, set to host:port of the imap server"); err != nil {
		return err
	}

	if err := validateNonEmptyStringField(c.User, "User must not be empty, set to username on the imap server"); err != nil {
		return err
	}

	if err := validateNonEmptyStringField(c.Password, "Password must not be empty, set to password of User on the imap server"); err != nil {
		return err
	}

	if err := validateNonEmptyStringField(c.SpamassassinHost, "SpamassassinHost must not be empty, set to host:port where spamassassin is reachable"); err != nil {
		return err
	}

	return nil
}

func validateNonEmptyStringField(field string, err string) error {
	if len(strings.TrimSpace(field)) == 0 {
		return errors.New(err)
	}

	return nil
}

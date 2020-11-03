// SPDX-License-Identifier: GPL-3.0-or-later
package imapassassin

import "fmt"

type ConfigFunc func(c *configuration) error

func DryRun() ConfigFunc {
	return func(c *configuration) error {
		c.DryRun = true

		return nil
	}
}

func DeleteSpam() ConfigFunc {
	return func(c *configuration) error {
		if c.MoveSpam {
			return fmt.Errorf("MoveSpam and DeleteSpam cannot be used at the same time")
		}

		c.DeleteSpam = true
		return nil
	}
}

func MoveSpam(spamMoveFolder string) ConfigFunc {
	return func(c *configuration) error {
		if len(spamMoveFolder) == 0 {
			return fmt.Errorf("SpamMoveFolder cannot be null")
		}

		if c.DeleteSpam {
			return fmt.Errorf("MoveSpam and DeleteSpam cannot be used at the same time")
		}

		c.MoveSpam = true
		c.SpamFolder = spamMoveFolder
		return nil
	}
}

func AppendReports(reportFolder string) ConfigFunc {
	return func(c *configuration) error {
		if len(reportFolder) == 0 {
			return fmt.Errorf("ReportFolder cannot be null")
		}
		c.AppendReports = true
		c.SpamReportFolder = reportFolder
		return nil
	}
}

func DeleteLearned() ConfigFunc {
	return func(c *configuration) error {
		c.DeleteLearned = true
		return nil
	}
}

type configuration struct {
	DryRun bool

	DeleteSpam    bool
	MoveSpam      bool
	AppendReports bool

	SpamFolder       string
	SpamReportFolder string

	DeleteLearned bool
}

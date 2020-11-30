// SPDX-License-Identifier: GPL-3.0-or-later
package imapassassin

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDryRun(t *testing.T) {
	cfg := &configuration{}
	err := DryRun()(cfg)

	assert.Equal(t, cfg, &configuration{DryRun: true})
	assert.Nil(t, err)
}

func TestDeleteSpam(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *configuration
		expected      *configuration
		expectedError error
	}{
		{"ok", &configuration{}, &configuration{DeleteSpam: true}, nil},
		{"moveconflict", &configuration{MoveSpam: true}, nil, fmt.Errorf("MoveSpam and DeleteSpam cannot be used at the same time")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := DeleteSpam()(tc.cfg)
			if tc.expected != nil {
				assert.Equal(t, tc.expected, tc.cfg)
				assert.Nil(t, err)
			} else {
				assert.Equal(t, tc.expectedError, err)
			}
		})
	}
}

func TestMoveSpam(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		cfg           *configuration
		expected      *configuration
		expectedError error
	}{
		{"ok", "spam", &configuration{}, &configuration{MoveSpam: true, SpamFolder: "spam"}, nil},
		{"lenvalidation", "", &configuration{}, nil, fmt.Errorf("SpamMoveFolder cannot be null")},
		{"deleteconflict", "spam", &configuration{DeleteSpam: true}, nil, fmt.Errorf("MoveSpam and DeleteSpam cannot be used at the same time")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := MoveSpam(tc.input)(tc.cfg)
			if tc.expected != nil {
				assert.Equal(t, tc.expected, tc.cfg)
				assert.Nil(t, err)
			} else {
				assert.Equal(t, tc.expectedError, err)
			}
		})
	}
}

func TestAppendReports(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		cfg           *configuration
		expected      *configuration
		expectedError error
	}{
		{"ok", "reports", &configuration{}, &configuration{AppendReports: true, SpamReportFolder: "reports"}, nil},
		{"lenvalidation", "", &configuration{}, nil, fmt.Errorf("ReportFolder cannot be null")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := AppendReports(tc.input)(tc.cfg)
			if tc.expected != nil {
				assert.Equal(t, tc.expected, tc.cfg)
				assert.Nil(t, err)
			} else {
				assert.Equal(t, tc.expectedError, err)
			}
		})
	}
}

func TestDeleteLearned(t *testing.T) {
	cfg := &configuration{}
	err := DeleteLearned()(cfg)

	assert.Equal(t, cfg, &configuration{DeleteLearned: true})
	assert.Nil(t, err)
}

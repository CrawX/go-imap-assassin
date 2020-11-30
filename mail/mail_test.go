// SPDX-License-Identifier: GPL-3.0-or-later
package mail

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMailHeaderInfos(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		hash    string
		err     string
	}{
		{"nonascii.msg", "M¥ RêÐ Çå§ïñð", "9dab633b491d5ed31e546d3e2396874da5eebb2503efbfff12241ab2b82824bd", ""},
		{"noreceived.msg", "Saying Hello", "db58509a9edd75ac0d9cddb528b5a2515df34336aeda5e416d412e3595b4e6d4", ""},
		{"nohashheaders.msg", "", "", "Received and Message-Id header header not found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rawMail, err := ioutil.ReadFile(path.Join("testdata", tc.name))
			assert.NoError(t, err)
			subject, hash, err := MailHeaderInfos(rawMail)

			if len(tc.err) == 0 {
				assert.NoError(t, err)
				assert.Equal(t, tc.subject, subject)
				assert.Equal(t, tc.hash, hash)
			} else {
				assert.Empty(t, subject)
				assert.Empty(t, hash)
				assert.EqualError(t, err, tc.err)
			}
		})
	}
}

func TestUnwrapSpamassassinReport(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		err      string
	}{
		{"nonascii.msg", "", ""},
		{"noreceived.msg", "", ""},
		{"nohashheaders.msg", "", ""},
		{"noreceived_wrapped.msg", "noreceived.msg", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rawMail, err := ioutil.ReadFile(path.Join("testdata", tc.name))
			assert.NoError(t, err)

			result, err := UnwrapSpamassassinReport(rawMail)
			assert.NoError(t, err)
			if len(tc.err) == 0 {
				expected := rawMail
				if len(tc.expected) > 0 {
					expected, err = ioutil.ReadFile(path.Join("testdata", tc.expected))
					assert.NoError(t, err)
				}
				assert.Equal(t, expected, result)
			}
		})
	}
}

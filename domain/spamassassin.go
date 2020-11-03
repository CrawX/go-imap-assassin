// SPDX-License-Identifier: GPL-3.0-or-later
package domain

type LearnType string

const (
	LearnSpam = LearnType("spam")
	LearnHam  = LearnType("ham")
)

type SpamResult struct {
	IsSpam bool
	Score  float64
	Body   []byte
	Error  error
}

type Spamassassin interface {
	Check(rawMail []byte) *SpamResult
	CheckAll(mails [][]byte, concurrency int) []*SpamResult
	Learn(learnType LearnType, rawMail []byte) error
	LearnAll(learnType LearnType, mails [][]byte, concurrency int) []error
}

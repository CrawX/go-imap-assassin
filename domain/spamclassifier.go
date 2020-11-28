// SPDX-License-Identifier: GPL-3.0-or-later

//go:generate mockgen -destination=mocks/spamclassifier.go -package=mocks . SpamClassifier,ConcurrentSpamClassifier
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

type SpamClassifier interface {
	Check(rawMail []byte) *SpamResult
	Learn(learnType LearnType, rawMail []byte) error
}

type ConcurrentSpamClassifier interface {
	CheckAll(mails [][]byte, concurrency int) []*SpamResult
	LearnAll(learnType LearnType, mails [][]byte, concurrency int) []error
}

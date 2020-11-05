// SPDX-License-Identifier: GPL-3.0-or-later
package classifier

import "github.com/CrawX/go-imap-assassin/domain"

type GoRoutineSpamClassifier struct {
	domain.SpamClassifier
}

func (grsc *GoRoutineSpamClassifier) CheckAll(mails [][]byte, concurrency int) []*domain.SpamResult {
	semaphore := make(chan bool, concurrency)
	results := make([]*domain.SpamResult, len(mails))
	for i := 0; i < len(mails); i++ {
		semaphore <- true
		go func(index int) {
			results[index] = grsc.Check(mails[index])
			if results[index].Error != nil {
				results[index] = grsc.Check(mails[index])
			}
			<-semaphore
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		semaphore <- true
	}

	return results
}

func (grsc *GoRoutineSpamClassifier) LearnAll(learnType domain.LearnType, mails [][]byte, concurrency int) []error {
	semaphore := make(chan bool, concurrency)
	results := make([]error, len(mails))
	for i := 0; i < len(mails); i++ {
		semaphore <- true
		go func(index int) {
			results[index] = grsc.Learn(learnType, mails[index])
			if results[index] != nil {
				results[index] = grsc.Learn(learnType, mails[index])
			}
			<-semaphore
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		semaphore <- true
	}

	return results
}

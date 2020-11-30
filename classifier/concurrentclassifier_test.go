// SPDX-License-Identifier: GPL-3.0-or-later
package classifier

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/CrawX/go-imap-assassin/domain"
	"github.com/CrawX/go-imap-assassin/domain/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func Test_CheckAllConcurrent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	classifier := mocks.NewMockSpamClassifier(ctrl)

	mail1, mail2, mail3 := []byte{0}, []byte{1}, []byte{2}
	errResult := &domain.SpamResult{Error: errors.New("error")}
	result1, result3 := &domain.SpamResult{Body: mail1}, &domain.SpamResult{Body: mail3}

	wg := &sync.WaitGroup{}
	wg.Add(3)

	// Mail1 is OK (no error)
	classifier.EXPECT().Check(gomock.Eq(mail1)).DoAndReturn(func(_ []byte) *domain.SpamResult {
		wg.Done()
		wg.Wait()
		return result1
	})

	// Mail2 returns an error, the retry still returns the error
	classifier.EXPECT().Check(gomock.Eq(mail2)).DoAndReturn(func(_ []byte) *domain.SpamResult {
		wg.Done()
		wg.Wait()
		return errResult
	})
	classifier.EXPECT().Check(gomock.Eq(mail2)).DoAndReturn(func(_ []byte) *domain.SpamResult {
		return errResult
	})

	// Mail3 returns an error, the retry is ok
	classifier.EXPECT().Check(gomock.Eq(mail3)).DoAndReturn(func(_ []byte) *domain.SpamResult {
		wg.Done()
		wg.Wait()
		return errResult
	})
	classifier.EXPECT().Check(gomock.Eq(mail3)).DoAndReturn(func(_ []byte) *domain.SpamResult {
		return result3
	})

	goRoutineSpamClassifier := GoRoutineSpamClassifier{classifier}

	resultsChan := make(chan []*domain.SpamResult)
	go func() {
		resultsChan <- goRoutineSpamClassifier.CheckAll([][]byte{mail1, mail2, mail3}, 3)
	}()

	timeoutChan := time.After(time.Millisecond * 50)
	select {
	case results := <-resultsChan:
		assert.Len(t, results, 3, "aggregated results should have a length of 2")
		assert.Equal(t, result1, results[0], "mail1 should not have caused errors")
		assert.Equal(t, errResult, results[1], "mail2 should still return the error after retry")
		assert.Equal(t, result3, results[2], "mail3 should be ok after retry")
	case <-timeoutChan:
		assert.Fail(t, "timeout when checking mails concurrently")
	}

}

func Test_LearnAllConcurrent(t *testing.T) {
	for _, learnType := range []domain.LearnType{domain.LearnSpam, domain.LearnHam} {
		t.Run(string(learnType), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			classifier := mocks.NewMockSpamClassifier(ctrl)

			mail1, mail2, mail3 := []byte{0}, []byte{1}, []byte{2}

			wg := &sync.WaitGroup{}
			wg.Add(3)

			// Mail1 is OK (no error)
			classifier.EXPECT().Learn(gomock.Eq(learnType), gomock.Eq(mail1)).DoAndReturn(func(_ domain.LearnType, _ []byte) error {
				wg.Done()
				wg.Wait()
				return nil
			})

			// Mail2 returns an error, the retry still returns the error
			err := errors.New("error")
			classifier.EXPECT().Learn(gomock.Eq(learnType), gomock.Eq(mail2)).DoAndReturn(func(_ domain.LearnType, _ []byte) error {
				wg.Done()
				wg.Wait()
				return err
			})
			classifier.EXPECT().Learn(gomock.Eq(learnType), gomock.Eq(mail2)).DoAndReturn(func(_ domain.LearnType, _ []byte) error {
				return err
			})

			// Mail3 returns an error, the retry is ok
			classifier.EXPECT().Learn(gomock.Eq(learnType), gomock.Eq(mail3)).DoAndReturn(func(_ domain.LearnType, _ []byte) error {
				wg.Done()
				wg.Wait()
				return err
			})
			classifier.EXPECT().Learn(gomock.Eq(learnType), gomock.Eq(mail3)).DoAndReturn(func(_ domain.LearnType, _ []byte) error {
				return nil
			})

			goRoutineSpamClassifier := GoRoutineSpamClassifier{classifier}

			resultsChan := make(chan []error)
			go func() {
				resultsChan <- goRoutineSpamClassifier.LearnAll(learnType, [][]byte{mail1, mail2, mail3}, 3)
			}()

			timeoutChan := time.After(time.Millisecond * 50)
			select {
			case results := <-resultsChan:
				assert.Len(t, results, 3, "aggregated results should have a length of 3")
				assert.Nil(t, results[0], "mail1 should not have caused errors")
				assert.Equal(t, err, results[1], "mail2 should still return the error after retry")
				assert.Nil(t, results[2], "mail3 should be ok after retry")
			case <-timeoutChan:
				assert.Fail(t, "timeout when checking mails concurrently")
			}
		})
	}
}

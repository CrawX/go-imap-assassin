// SPDX-License-Identifier: GPL-3.0-or-later
package spamassassin

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/CrawX/go-imap-assassin/domain"
	"github.com/CrawX/go-imap-assassin/mail"

	"github.com/teamwork/spamc"
)

const SpamassassinTimeout = 20 * time.Second

type Spamassassin struct {
	client *spamc.Client
}

func NewSpamassassin(host string) (*Spamassassin, error) {
	client := spamc.New(host, &net.Dialer{
		Timeout: SpamassassinTimeout,
	})
	err := client.Ping(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("could not ping spamassassin: %w", err)
	}

	return &Spamassassin{client: client}, nil
}

func (sa *Spamassassin) Check(rawMail []byte) *domain.SpamResult {
	out, err := sa.client.Process(context.TODO(), bytes.NewReader(rawMail), nil)
	if err != nil {
		return errResult(fmt.Errorf("could not check spamassassin: %w", err))
	}

	var body []byte
	if out.IsSpam {
		body, err = ioutil.ReadAll(out.Message)
		if err != nil {
			return errResult(fmt.Errorf("could not read response body: %w", err))
		}
	}

	err = out.Message.Close()
	if err != nil {
		return errResult(fmt.Errorf("could not close response: %w", err))
	}

	return &domain.SpamResult{
		IsSpam: out.IsSpam,
		Score:  out.Score,
		Body:   body,
	}
}

func (sa *Spamassassin) CheckAll(mails [][]byte, concurrency int) []*domain.SpamResult {
	semaphore := make(chan bool, concurrency)
	results := make([]*domain.SpamResult, len(mails))
	for i := 0; i < len(mails); i++ {
		semaphore <- true
		go func(index int) {
			results[index] = sa.Check(mails[index])
			if results[index].Error != nil {
				results[index] = sa.Check(mails[index])
			}
			<-semaphore
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		semaphore <- true
	}

	return results
}

func (sa *Spamassassin) Learn(learnType domain.LearnType, rawMail []byte) error {
	header := spamc.Header{}.Set("Set", "local")
	switch learnType {
	case domain.LearnSpam:
		header = header.Set("Message-class", "spam")
	case domain.LearnHam:
		header = header.Set("Message-class", "ham")
	default:
		return fmt.Errorf("unsupported learn type %v", learnType)
	}

	unwrapped, err := mail.UnwrapSpamassassinReport(rawMail)
	if err != nil {
		return fmt.Errorf("could not unwrap spamassassin report: %w", err)
	}
	_, err = sa.client.Tell(context.TODO(), bytes.NewReader(unwrapped), header)
	if err != nil {
		return fmt.Errorf("could not learn spamsassassin: %w", err)
	}
	return nil
}

func (sa *Spamassassin) LearnAll(learnType domain.LearnType, mails [][]byte, concurrency int) []error {
	semaphore := make(chan bool, concurrency)
	results := make([]error, len(mails))
	for i := 0; i < len(mails); i++ {
		semaphore <- true
		go func(index int) {
			results[index] = sa.Learn(learnType, mails[index])
			if results[index] != nil {
				results[index] = sa.Learn(learnType, mails[index])
			}
			<-semaphore
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		semaphore <- true
	}

	return results
}

func errResult(err error) *domain.SpamResult {
	return &domain.SpamResult{Error: err}
}

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

const SpamAssassinTimeout = 20 * time.Second

type SpamAssassin struct {
	client *spamc.Client
}

func NewSpamassassin(host string) (*SpamAssassin, error) {
	client := spamc.New(host, &net.Dialer{
		Timeout: SpamAssassinTimeout,
	})
	err := client.Ping(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("could not ping SpamAssassin: %w", err)
	}

	return &SpamAssassin{client: client}, nil
}

func (sa *SpamAssassin) Check(rawMail []byte) *domain.SpamResult {
	out, err := sa.client.Process(context.TODO(), bytes.NewReader(rawMail), nil)
	if err != nil {
		return errResult(fmt.Errorf("could not check SpamAssassin: %w", err))
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

func (sa *SpamAssassin) Learn(learnType domain.LearnType, rawMail []byte) error {
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
		return fmt.Errorf("could not unwrap SpamAssassin-style report: %w", err)
	}
	_, err = sa.client.Tell(context.TODO(), bytes.NewReader(unwrapped), header)
	if err != nil {
		return fmt.Errorf("could not learn SpamAssassin: %w", err)
	}
	return nil
}

func errResult(err error) *domain.SpamResult {
	return &domain.SpamResult{Error: err}
}

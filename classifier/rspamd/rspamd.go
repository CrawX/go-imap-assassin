// SPDX-License-Identifier: GPL-3.0-or-later
package rspamd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/CrawX/go-imap-assassin/domain"
	"github.com/CrawX/go-imap-assassin/mail"
)

const RspamdTimeout = 20 * time.Second

// gathered via trial&error and the source-code of various rspamd modules. These are caused by misconfiguration on the
// sender's side and not by the dns server being slow to respond for example.
var okFailSymbols = regexp.MustCompile(`^(R_DKIM_PERMFAIL|DMARC_POLICY_SOFTFAIL|R_SPF_SOFTFAIL|DMARC_DNSFAIL|R_SPF_FAIL)$`)

type Rspamd struct {
	client   *http.Client
	host     string
	password string
}

func NewRspamd(host, password string) (*Rspamd, error) {
	rspamd := &Rspamd{
		client: &http.Client{
			Timeout: RspamdTimeout,
		},
		host:     host,
		password: password,
	}
	err := rspamd.Ping()
	if err != nil {
		return nil, fmt.Errorf("could not ping rspamd: %w", err)
	}

	return rspamd, nil
}

func (rs *Rspamd) Ping() error {
	resp, err := rs.client.Get(rs.host + "/ping")
	if err != nil {
		return fmt.Errorf("could not ping rspamd: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from rspamd, expected 200", resp.StatusCode)
	}
	defer resp.Body.Close()

	return nil
}

type checkResponse struct {
	IsSkipped bool    `json:"is_skipped"`
	Score     float64 `json:"score"`
	Symbols   map[string]struct {
		Name  string
		Score float64
	} `json:"symbols"`
	Action string `json:"action"`
}

func (rs *Rspamd) Check(rawMail []byte) *domain.SpamResult {
	req, err := http.NewRequest(http.MethodPost, rs.host+"/checkv2", bytes.NewReader(rawMail))
	if err != nil {
		return errResult(fmt.Errorf("could not create check request: %w", err))
	}

	resp, err := rs.doAuthenticated(req)
	if err != nil {
		return errResult(fmt.Errorf("could not perform check request: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Errorf("unexpected status %d from rspamd, expected 200", resp.StatusCode))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errResult(fmt.Errorf("could not read rspamd response: %w", err))
	}

	checkResponse := &checkResponse{}
	err = json.Unmarshal(body, checkResponse)
	if err != nil {
		return errResult(fmt.Errorf("could not deserialize rspamd response: %w", err))
	}

	if len(checkResponse.Symbols) == 0 {
		return errResult(fmt.Errorf("could not find any symbols in rspamd response"))
	}

	for symbol := range checkResponse.Symbols {
		if strings.HasSuffix(symbol, "FAIL") && !okFailSymbols.MatchString(symbol) {
			return errResult(fmt.Errorf("unexpected FAIL symbol %s in rspamd response", symbol))
		}
	}

	result := &domain.SpamResult{
		IsSpam: checkResponse.Action != "no action",
		Score:  checkResponse.Score,
	}

	if result.IsSpam {
		result.Body, err = report(rawMail, body, result.Score)
		if err != nil {
			return errResult(fmt.Errorf("could not create report: %w", err))
		}
	}

	return result
}

func (rs *Rspamd) Learn(learnType domain.LearnType, rawMail []byte) error {
	suffix := ""
	switch learnType {
	case domain.LearnSpam:
		suffix = "learnspam"
	case domain.LearnHam:
		suffix = "learnham"
	default:
		return fmt.Errorf("unsupported learn type %v", learnType)
	}

	unwrapped, err := mail.UnwrapSpamassassinReport(rawMail)
	if err != nil {
		return fmt.Errorf("could not unwrap SpamAssassin-style report: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, rs.host+"/"+suffix, bytes.NewReader(unwrapped))
	if err != nil {
		return fmt.Errorf("could not create learn request: %w", err)
	}

	resp, err := rs.doAuthenticated(req)
	if err != nil {
		return fmt.Errorf("could not perform learn request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAlreadyReported && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d from rspamd, expected 200/204/208", resp.StatusCode)
	}

	return nil
}

func (rs *Rspamd) doAuthenticated(req *http.Request) (*http.Response, error) {
	req.Header.Set("Password", rs.password)
	resp, err := rs.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("could not send request to rspamd: %w", err)
	}

	return resp, nil
}

func errResult(err error) *domain.SpamResult {
	return &domain.SpamResult{Error: err}
}

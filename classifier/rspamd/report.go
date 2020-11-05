// SPDX-License-Identifier: GPL-3.0-or-later
package rspamd

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	mailutil "github.com/CrawX/go-imap-assassin/mail"
	"github.com/emersion/go-message/mail"
)

// Report creates a report similar to SpamAssassin's report but based on rspamd's data.
func report(rawMail []byte, response []byte, score float64) ([]byte, error) {
	subject, _, err := mailutil.MailHeaderInfos(rawMail)
	if err != nil {
		return nil, fmt.Errorf("could not read mail: %w", err)
	}

	buffer := &bytes.Buffer{}

	from := []*mail.Address{{Name: "go-imap-assassin", Address: "go-imap-assassin@localhost"}}
	to := []*mail.Address{{Name: "go-imap-assassin", Address: "go-imap-assassin@localhost"}}

	header := mail.Header{}
	header.SetDate(time.Now())
	header.SetAddressList("From", from)
	header.SetAddressList("To", to)
	header.SetSubject(subject)
	header.Set("X-Spam-Checker-Version", "rspamd")
	header.Set("X-Spam-Flag", strings.Repeat("*", int(math.Round(score))))
	header.Set("X-Spam-Status", fmt.Sprintf("X-Spam-Status: Yes, score=%.1f", score))

	mailWriter, err := mail.CreateWriter(buffer, header)
	if err != nil {
		return nil, fmt.Errorf("could not create mail writer: %w", err)
	}

	textPart, err := mailWriter.CreateInline()
	if err != nil {
		return nil, fmt.Errorf("could not create mail text part: %w", err)
	}
	inlineHeader := mail.InlineHeader{}
	inlineHeader.Set("Content-Type", "text/plain")
	textPartWriter, err := textPart.CreatePart(inlineHeader)
	if err != nil {
		return nil, fmt.Errorf("could not create text part: %w", err)
	}
	_, err = io.WriteString(textPartWriter, "rspamd has created the following report for the attached spam mail:\n\n")
	if err != nil {
		return nil, fmt.Errorf("could not write text part: %w", err)
	}
	_, err = textPartWriter.Write(response)
	if err != nil {
		return nil, fmt.Errorf("could not write json response to text part: %w", err)
	}
	err = textPartWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("could not close text part writer: %w", err)
	}
	err = textPart.Close()
	if err != nil {
		return nil, fmt.Errorf("could not close text part: %w", err)
	}

	attachmentHeader := mail.AttachmentHeader{}
	attachmentHeader.Set("Content-Type", "message/rfc822; x-spam-type=original")
	attachmentHeader.Set("Content-Description", "original message before rspamd")
	attachmentHeader.Set("Content-Transfer-Encoding", "binary")
	attachmentHeader.SetFilename("original-mail.eml")
	attachmentWriter, err := mailWriter.CreateAttachment(attachmentHeader)
	if err != nil {
		return nil, fmt.Errorf("could not create attachment part: %w", err)
	}
	_, err = attachmentWriter.Write(rawMail)
	if err != nil {
		return nil, fmt.Errorf("could not write attachment: %w", err)
	}
	err = attachmentWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("could not close attachment writer: %w", err)
	}

	err = mailWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("could not close mail writer: %w", err)
	}

	return buffer.Bytes(), nil
}

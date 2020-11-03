// SPDX-License-Identifier: GPL-3.0-or-later
package mail

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	stdmail "net/mail"
	"strings"

	"github.com/emersion/go-message/charset"
)

func MailHeaderInfos(rawMail []byte) (string, string, error) {
	msg, err := stdmail.ReadMessage(bytes.NewReader(rawMail))
	if err != nil {
		return "", "", fmt.Errorf("could not parse mail: %w", err)
	}

	messageIdHeader := msg.Header["Message-Id"]
	receivedHeader := msg.Header["Received"]
	if len(receivedHeader) == 0 && len(messageIdHeader) == 0 {
		return "", "", fmt.Errorf("Received and Message-Id header header not found")
	}

	subjectHeader := msg.Header.Get("Subject")

	dec := &mime.WordDecoder{
		CharsetReader: charset.Reader,
	}
	subject, err := dec.DecodeHeader(subjectHeader)
	if err != nil {
		return "", "", fmt.Errorf("could decode subject header: %w", err)
	}

	mailIdHash, err := hash([][]string{messageIdHeader, receivedHeader})
	if err != nil {
		return "", "", fmt.Errorf("could not hash headers: %w", err)
	}

	return subject, mailIdHash, nil
}

func UnwrapSpamassassinReport(rawMail []byte) ([]byte, error) {
	msg, err := stdmail.ReadMessage(bytes.NewReader(rawMail))
	if err != nil {
		return nil, fmt.Errorf("could not parse mail: %w", err)
	}

	contentType := msg.Header.Get("Content-Type")
	if len(contentType) == 0 {
		return rawMail, nil
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return rawMail, nil
	}

	if !strings.HasPrefix(mediaType, "multipart/") {
		return rawMail, nil
	}

	saHeaders := 0
	for key := range msg.Header {
		if strings.Contains(key, "X-Spam-") {
			saHeaders++
		}
	}

	if saHeaders < 2 {
		return rawMail, nil
	}

	mr := multipart.NewReader(msg.Body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return rawMail, nil
		}
		if err != nil {
			return nil, fmt.Errorf("unexpected error while unwrapping: %w", err)
		}

		if strings.Contains(p.Header.Get("Content-Type"), "x-spam-type=original") {
			unwrapped, err := ioutil.ReadAll(p)
			if err != nil {
				return nil, fmt.Errorf("unexpected error while reading wrapped body: %w", err)
			}

			return unwrapped, nil
		}
	}
}

func ShortSubject(subject string) string {
	if (len(subject)) > 30 {
		subject = subject[:30] + "..."
	}
	return subject
}

func hash(input [][]string) (string, error) {
	sha := sha256.New()
	for _, i := range input {
		for _, ii := range i {
			_, err := sha.Write([]byte(ii))
			if err != nil {
				return "", fmt.Errorf("could not hash: %w", err)
			}
		}
	}

	return fmt.Sprintf("%x", sha.Sum(nil)), nil
}

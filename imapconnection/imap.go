// SPDX-License-Identifier: GPL-3.0-or-later
package imapconnection

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/CrawX/go-imap-assassin/domain"
	"github.com/CrawX/go-imap-assassin/log"
	"github.com/CrawX/go-imap-assassin/mail"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-move"
	"github.com/emersion/go-imap-uidplus"
	"github.com/emersion/go-imap/client"
	"github.com/sirupsen/logrus"
)

type ImapConnection struct {
	connection  *client.Client
	mailDeleter deleter
	mailMover   mover

	server, user, password string

	selectedFolder string

	l *logrus.Logger
}

func NewImapConnection(server string, user string, password string) (*ImapConnection, error) {
	imapClient, err := client.DialTLS(server, nil)
	if err != nil {
		return nil, fmt.Errorf("could not dial to imap: %w", err)
	}

	err = imapClient.Login(user, password)
	if err != nil {
		return nil, fmt.Errorf("could not login to imap: %w", err)
	}

	uidPlusClient := uidplus.NewClient(imapClient)
	uidPlusSupported, err := uidPlusClient.SupportUidPlus()
	if err != nil {
		return nil, fmt.Errorf("could not check for UIDPLUS support: %w", err)
	}

	moveClient := move.NewClient(imapClient)
	moveSupported, err := moveClient.SupportMove()
	if err != nil {
		return nil, fmt.Errorf("could not check for MOVE support: %w", err)
	}

	conn := &ImapConnection{
		connection: imapClient,
		server:     server,
		user:       user,
		password:   password,
		l:          log.Logger(log.LOG_IMAP),
	}

	baseLogger := conn.l.WithFields(logrus.Fields{"server": server})
	baseLogger.Debug("Logged in to server")

	if uidPlusSupported {
		baseLogger.Debug("UIDPLUS supported on server, using UID delete")
		conn.mailDeleter = &uidPlusDeleter{
			imapConn:      conn,
			uidplusClient: uidPlusClient,
		}
	} else {
		baseLogger.Info("UIDPLUS not supported on server, falling back to flag&expunge")
		conn.mailDeleter = &compatibilityDeleter{
			imapConn: conn,
		}
	}

	if moveSupported {
		baseLogger.Debug("MOVE supported on server")
		conn.mailMover = &moveMover{
			moveClient: moveClient,
		}
	} else {
		baseLogger.Info("MOVE not supported on server, falling back to copy&delete")
		if uidPlusSupported {
			baseLogger.Debug("UIDPLUS supported on server, using UID delete for copy")
		} else {
			baseLogger.Info("UIDPLUS not supported on server, falling back to flag&expunge for copy")
		}
		conn.mailMover = &compatibilityMover{
			imapConn: conn,
		}
	}

	return conn, nil
}

func (ic *ImapConnection) Select(folder string) (uint32, error) {
	m, err := ic.connection.Select(folder, false)
	if err != nil {
		return 0, fmt.Errorf("could not select folder: %w", err)
	}

	ic.selectedFolder = folder
	return m.UidValidity, nil
}

func (ic *ImapConnection) ListUids() ([]uint32, error) {
	// Get all UIDs in folder (empty search criteria)
	criteria := imap.NewSearchCriteria()
	ids, err := ic.connection.UidSearch(criteria)
	if err != nil {
		return nil, fmt.Errorf("could not list folder: %w", err)
	}

	return ids, nil
}

func (ic *ImapConnection) FetchMails(uids []uint32) ([]*domain.RawImapMail, error) {
	seqset := &imap.SeqSet{}
	seqset.AddNum(uids...)

	messages := make(chan *imap.Message, 10)
	fullBodySection := &imap.BodySectionName{
		Peek: true,
	}

	fetchItems := []imap.FetchItem{fullBodySection.FetchItem()}
	done := make(chan error, 1)
	go func() {
		done <- ic.connection.UidFetch(seqset, fetchItems, messages)
	}()

	mails := []*domain.RawImapMail{}
	for msg := range messages {
		r := msg.GetBody(fullBodySection)
		if r == nil {
			fmt.Println(msg)
		}
		rawBody, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("could not read mail body: %w", err)
		}

		subject, mailIdHash, err := mail.MailHeaderInfos(rawBody)
		if err != nil {
			return nil, fmt.Errorf("could not parse mail header infos: %w", err)
		}

		mails = append(
			mails,
			&domain.RawImapMail{
				Uid:        msg.Uid,
				Subject:    subject,
				MailIdHash: mailIdHash,
				RawMail:    rawBody,
			},
		)
	}

	err := <-done
	if err != nil {
		return nil, fmt.Errorf("could not fetch mails: %w", err)
	}

	return mails, nil
}

func (ic *ImapConnection) FetchIdHeaders(uids []uint32) ([]*domain.ImapIdInfo, error) {
	seqset := &imap.SeqSet{}
	seqset.AddNum(uids...)
	section := &imap.BodySectionName{
		BodyPartName: imap.BodyPartName{
			Specifier: imap.HeaderSpecifier,
			Fields: []string{
				"Received",
				"Message-Id",
				"Subject",
			},
		},
		Peek: true,
	}
	fetchItems := []imap.FetchItem{section.FetchItem()}

	out := make(chan *imap.Message)
	done := make(chan error, 1)
	go func() {
		done <- ic.connection.UidFetch(seqset, fetchItems, out)
	}()

	results := []*domain.ImapIdInfo{}
	for msg := range out {
		r := msg.GetBody(section)

		rawHeaders, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("could not read mail body: %w", err)
		}

		subject, mailIdHash, err := mail.MailHeaderInfos(rawHeaders)
		if err != nil {
			return nil, fmt.Errorf("could not parse mail header infos: %w", err)
		}

		results = append(
			results,
			&domain.ImapIdInfo{
				Uid:        msg.Uid,
				Subject:    subject,
				MailIdHash: mailIdHash,
			},
		)
	}

	err := <-done
	if err != nil {
		return nil, fmt.Errorf("could not fetch mails: %w", err)
	}

	return results, nil
}

func (ic *ImapConnection) Close() error {
	return ic.connection.Logout()
}

type RawMail struct {
	Uid  uint32
	Data []byte
}

func (ic *ImapConnection) Check(ids []uint32) ([]*RawMail, error) {
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	seqset := new(imap.SeqSet)
	for _, v := range ids {
		seqset.AddNum(v)
	}

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		done <- ic.connection.UidFetch(seqset, items, messages)
	}()

	results := []*RawMail{}
	for {
		select {
		case err := <-done:
			if err != nil {
				return nil, fmt.Errorf("imap command to ListUids failed: %w", err)
			}
			done = nil
		case msg, ok := <-messages:
			if !ok {
				done = nil
				messages = nil
				break
			}

			r := msg.GetBody(section)
			rawMail, err := ioutil.ReadAll(r)
			if err != nil {
				return nil, fmt.Errorf("could not read mail: %w", err)
			}

			results = append(
				results,
				&RawMail{
					msg.Uid,
					rawMail,
				},
			)
		}

		if done == nil && messages == nil {
			break
		}
	}

	return results, nil
}

func (ic *ImapConnection) Put(body []byte, folder string) error {
	err := ic.connection.Append(folder, nil, time.Now(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("could not append: %w", err)
	}

	return nil
}

func (ic *ImapConnection) Delete(uids []uint32) error {
	return ic.mailDeleter.delete(uids)
}

func (ic *ImapConnection) DeleteReady() (error, error) {
	return ic.mailDeleter.deleteReady()
}

func (ic *ImapConnection) flagDeleted(uids []uint32) (*imap.SeqSet, error) {
	seqset := &imap.SeqSet{}
	seqset.AddNum(uids...)
	err := ic.connection.UidStore(seqset, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil)
	if err != nil {
		return nil, fmt.Errorf("could set delete flag: %w", err)
	}

	return seqset, nil
}

func (ic *ImapConnection) MoveReady() (error, error) {
	return ic.mailMover.moveReady()
}

func (ic *ImapConnection) Move(uids []uint32, folder string) error {
	return ic.mailMover.move(uids, folder)
}

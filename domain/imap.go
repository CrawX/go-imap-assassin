// SPDX-License-Identifier: GPL-3.0-or-later
package domain

//go:generate mockgen -destination=mocks/imap.go -package=mocks . ImapConnector
type RawImapMail struct {
	Uid        uint32
	Subject    string
	MailIdHash string
	RawMail    []byte
}

type ImapIdInfo struct {
	Uid        uint32
	Subject    string
	MailIdHash string
}

type ImapConnector interface {
	Select(folder string) (uint32, error)
	ListUids() ([]uint32, error)
	FetchMails(uids []uint32) ([]*RawImapMail, error)
	FetchIdHeaders(uids []uint32) ([]*ImapIdInfo, error)
	Put(body []byte, folder string) error
	DeleteReady() (error, error)
	Delete(uids []uint32) error
	MoveReady() (error, error)
	Move(uids []uint32, folder string) error

	Close() error
}

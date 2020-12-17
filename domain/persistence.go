// SPDX-License-Identifier: GPL-3.0-or-later
package domain

//go:generate mockgen -destination=mocks/persistence.go -package=mocks . Persistence
type ImapFolder struct {
	Name        string
	UidValidity uint32
}

type MailClass int

const (
	Checked     = MailClass(0)
	LearnedSpam = MailClass(10)
	LearnedHam  = MailClass(11)
)

type SavedImapMail struct {
	Id         int64
	Class      MailClass
	Uid        uint32
	MailIdHash string
	FolderName string
	Subject    string
	IsSpam     bool
	Score      float64
}

type SaveMail struct {
	Class      MailClass
	Uid        uint32
	MailIdHash string
	FolderName string
	Subject    string
	IsSpam     *bool
	Score      *float64
}

type Persistence interface {
	Close() error
	AllFolders() ([]*ImapFolder, error)
	SaveFolder(name string, uidValidity uint32) error
	GetMailsInFolder(class MailClass, folder string) ([]*SavedImapMail, error)
	FindMailByFolderHash(class MailClass, folder string, mailIdHash string) (*SavedImapMail, error)
	HashesExist(class MailClass, mailIdHashes []string) (map[string]bool, error)
	UpdateUid(id int64, uid uint32) error
	SaveMails(mails []SaveMail) error
}

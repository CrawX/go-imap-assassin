// SPDX-License-Identifier: GPL-3.0-or-later
package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/CrawX/go-imap-assassin/domain"
	"github.com/CrawX/go-imap-assassin/log"
	"github.com/CrawX/go-imap-assassin/persistence/migrations"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rubenv/sql-migrate"
	"github.com/sirupsen/logrus"
)

type Persistence struct {
	db *sqlx.DB
	l  *logrus.Logger
}

func NewPersistence(datasource string) (*Persistence, error) {
	db, err := sqlx.Connect("sqlite3", datasource)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	l := log.Logger(log.LOG_PERSISTENCE)
	l.WithField("file", datasource).Info("Connected")

	migrationSource := &migrate.HttpFileSystemMigrationSource{
		FileSystem: migrations.Dir(false, "/sql"),
	}

	_, err = db.Exec(`PRAGMA journal_mode=WAL`)
	if err != nil {
		return nil, fmt.Errorf("could not set journal mode: %w", err)
	}
	_, err = db.Exec(`PRAGMA synchronous=normal`)
	if err != nil {
		return nil, fmt.Errorf("could not set synchronous mode: %w", err)
	}

	appliedMigrations, err := migrate.Exec(db.DB, "sqlite3", migrationSource, migrate.Up)
	if err != nil {
		return nil, fmt.Errorf("could not migrate to newest version: %w", err)
	}

	l.WithField("migrations", appliedMigrations).Debug("Executed migrations")

	return &Persistence{
		db: db,
		l:  l,
	}, nil
}

func (p *Persistence) Close() error {
	err := p.db.Close()
	if err != nil {
		return fmt.Errorf("could not close db: %w", err)
	}
	p.l.Info("Disconnected")
	return nil
}

func (p *Persistence) AllFolders() ([]*domain.ImapFolder, error) {
	dbFolders := []struct {
		Name        string
		UidValidity uint32
	}{}

	err := p.db.Select(
		&dbFolders,
		`SELECT name, uidvalidity from folders`,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query db: %w", err)
	}

	folders := []*domain.ImapFolder{}
	for _, f := range dbFolders {
		folders = append(
			folders,
			&domain.ImapFolder{
				Name:        f.Name,
				UidValidity: f.UidValidity,
			},
		)
	}

	p.l.WithField("Count", len(folders)).Debug("Found folders")

	return folders, nil
}

func (p *Persistence) SaveFolder(name string, uidValidity uint32) error {
	_, err := p.db.Exec(
		"INSERT OR REPLACE INTO folders (name, uidvalidity) VALUES (?, ?)",
		name,
		uidValidity,
	)

	if err != nil {
		return fmt.Errorf("could not save folder: %w", err)
	}

	p.l.WithFields(logrus.Fields{"Name": name, "UidValidity": uidValidity}).Info("Persisted folder")
	return nil
}

func (p *Persistence) GetMailsInFolder(class domain.MailClass, folder string) ([]*domain.SavedImapMail, error) {
	dbMessages := []struct {
		Id         int64
		Class      int
		Uid        uint32
		MailIdHash string
		FolderName string
		Subject    string
		IsSpam     bool
		Score      float64
	}{}

	err := p.db.Select(
		&dbMessages,
		`SELECT id, class, uid, mailidhash, foldername, subject from messages WHERE class = ? AND foldername = ?`,
		int(class),
		folder,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query db: %w", err)
	}

	messages := []*domain.SavedImapMail{}
	for _, m := range dbMessages {
		messages = append(
			messages,
			&domain.SavedImapMail{
				Id:         m.Id,
				Class:      domain.MailClass(m.Class),
				Uid:        m.Uid,
				MailIdHash: m.MailIdHash,
				FolderName: m.FolderName,
				Subject:    m.Subject,
				IsSpam:     m.IsSpam,
				Score:      m.Score,
			},
		)
	}

	return messages, nil
}

func (p *Persistence) FindMailByFolderHash(class domain.MailClass, folder string, mailIdHash string) (*domain.SavedImapMail, error) {
	dbMail := struct {
		Id         int64
		Class      int
		Uid        uint32
		MailIdHash string
		FolderName string
		Subject    string
		IsSpam     bool
		Score      float64
	}{}

	err := p.db.Get(
		&dbMail,
		"SELECT id, class, uid, mailidhash, foldername, subject, isspam, score from messages WHERE class = ? AND foldername = ? AND mailidhash = ?",
		int(class),
		folder,
		mailIdHash,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not query db: %w", err)
	}

	return &domain.SavedImapMail{
		Id:         dbMail.Id,
		Class:      domain.MailClass(dbMail.Class),
		Uid:        dbMail.Uid,
		MailIdHash: dbMail.MailIdHash,
		FolderName: dbMail.FolderName,
		Subject:    dbMail.Subject,
		IsSpam:     dbMail.IsSpam,
		Score:      dbMail.Score,
	}, nil
}

func (p *Persistence) HashesExist(class domain.MailClass, mailIdHashes []string) (map[string]bool, error) {
	qry, args, err := sqlx.Named(
		"SELECT mailidhash from messages WHERE class = :class AND mailidhash IN (:hashes)",
		map[string]interface{}{
			"class":  int(class),
			"hashes": mailIdHashes,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not create query: %w", err)
	}

	qry, args, err = sqlx.In(qry, args...)
	if err != nil {
		return nil, fmt.Errorf("could not replace IN in query: %w", err)
	}

	hashes := []string{}
	err = p.db.Select(
		&hashes,
		qry,
		args...,
	)

	if err != nil {
		return nil, fmt.Errorf("could not query db: %w", err)
	}

	result := map[string]bool{}
	for _, hash := range hashes {
		result[hash] = true
	}

	return result, nil
}

func (p *Persistence) UpdateUid(id int64, uid uint32) error {
	result, err := p.db.Exec(
		"UPDATE messages set uid = ? WHERE id = ?",
		uid, id,
	)
	if err != nil {
		return fmt.Errorf("could not update uid: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("could not get num of affected rows: %w", err)
	}

	if affected != 1 {
		return fmt.Errorf("unexpected number of affected rows, expected 1 got %d", affected)

	}

	return nil
}

func (p *Persistence) SaveMails(mails []domain.SaveMail) error {
	tx, err := p.db.BeginTxx(context.TODO(), nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}

	stmt, err := tx.Prepare(
		"INSERT INTO messages(class, uid, mailidhash, foldername, subject, isspam, score) VALUES(?, ?, ?, ?, ?, ?, ?)",
	)
	if err != nil {
		return txEnd(tx, fmt.Errorf("could not prepare statement: %w", err))
	}

	for _, mail := range mails {
		_, err := stmt.Exec(
			mail.Class, mail.Uid, mail.MailIdHash, mail.FolderName, mail.Subject, mail.IsSpam, mail.Score,
		)

		if err != nil {
			return txEnd(tx, fmt.Errorf("could not save mail: %w", err))
		}
	}

	return txEnd(tx, nil)
}

func txEnd(tx *sqlx.Tx, err error) error {
	if err == nil {
		err = tx.Commit()
		if err != nil {
			return fmt.Errorf("could not commit tx: %w", err)
		}
	} else {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			errStr := err.Error()
			return fmt.Errorf("%s, could not rollback tx: %w", errStr, rollbackErr)
		} else {
			return err
		}
	}

	return nil
}

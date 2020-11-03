// SPDX-License-Identifier: GPL-3.0-or-later
package imapassassin

import (
	"fmt"
	"sort"
	"time"

	"github.com/CrawX/go-imap-assassin/domain"
	"github.com/CrawX/go-imap-assassin/log"
	"github.com/CrawX/go-imap-assassin/mail"

	"github.com/sirupsen/logrus"
)

const (
	BatchSize        = 50
	CheckConcurrency = 16
	LearnConcurrency = 8
)

type ImapAssassin struct {
	persistence    domain.Persistence
	spamassassin   domain.Spamassassin
	imapConnection domain.ImapConnector

	configuration *configuration

	l *logrus.Logger
}

func NewImapAssassin(persistence domain.Persistence, spamassassin domain.Spamassassin, imapConnection domain.ImapConnector, configFunc ...ConfigFunc) (*ImapAssassin, error) {
	config := &configuration{}
	for _, f := range configFunc {
		err := f(config)
		if err != nil {
			return nil, fmt.Errorf("error applying configuration: %w", err)
		}
	}

	return &ImapAssassin{
		persistence:    persistence,
		spamassassin:   spamassassin,
		imapConnection: imapConnection,
		configuration:  config,
		l:              log.Logger(log.LOG_IMAPASSASSIN),
	}, nil
}

func (ia *ImapAssassin) CheckSpam(folders []string) error {
	knownFolders, err := ia.persistence.AllFolders()
	if err != nil {
		return fmt.Errorf("could not list known folders: %w", err)
	}

	for _, f := range folders {
		uidvalidity, err := ia.imapConnection.Select(f)
		if err != nil {
			return fmt.Errorf("could not select folder %s: %w", f, err)
		}

		if !ia.configuration.DryRun {
			if ia.configuration.DeleteSpam {
				notDeleteReadyReason, err := ia.imapConnection.DeleteReady()
				if err != nil {
					return fmt.Errorf("could not check for delete readiness: %w", err)
				}

				if notDeleteReadyReason != nil {
					ia.l.WithFields(logrus.Fields{"folder": f, "error": notDeleteReadyReason}).Warn("Folder is not ready for mail deletion, skipping")
					continue
				}
			} else if ia.configuration.MoveSpam {
				notMoveReadyReason, err := ia.imapConnection.MoveReady()
				if err != nil {
					return fmt.Errorf("could not check for move readiness: %w", err)
				}

				if notMoveReadyReason != nil {
					ia.l.WithFields(logrus.Fields{"folder": f, "error": notMoveReadyReason}).Warn("Folder is not ready for mail moving, skipping")
					continue
				}
			}
		}

		newMailUids, err := ia.getNewMailUids(f, domain.Checked, knownFolders, uidvalidity)
		if err != nil {
			return fmt.Errorf("could not determine new mail uids: %w", err)
		}

		if len(newMailUids) == 0 {
			ia.l.WithFields(logrus.Fields{"folder": f, "newmails": len(newMailUids)}).Info("Folder contains no new mails")
			break
		}

		batches := partitionUids(newMailUids, BatchSize)
		ia.l.WithFields(logrus.Fields{"folder": f, "newmails": len(newMailUids), "batches": len(batches)}).Info("Found mails to check")

		totalOk, totalSpam := 0, 0
		for _, batch := range batches {
			start := time.Now()
			ia.l.WithFields(logrus.Fields{"batchsize": len(batch)}).Debug("Checking batch")
			mails, err := ia.imapConnection.FetchMails(batch)
			if err != nil {
				return fmt.Errorf("could not fetch mail batch: %w", err)
			}
			ia.l.WithFields(logrus.Fields{"duration": time.Since(start)}).Debug("Fetched mail batch")
			rawMails := make([][]byte, len(mails))
			for i := 0; i < len(mails); i++ {
				rawMails[i] = mails[i].RawMail
			}
			spamResults := ia.spamassassin.CheckAll(rawMails, CheckConcurrency)

			// Split spam and ham, append reports
			ok, spam := []uint32{}, []uint32{}
			for i, m := range mails {
				result := spamResults[i]
				if result.Error != nil {
					return fmt.Errorf(`Could not check mail "%s (%v)": %w`, mail.ShortSubject(m.Subject), m.Uid, result.Error)
				}

				ia.l.WithFields(logrus.Fields{"folder": f, "subject": mail.ShortSubject(m.Subject), "isSpam": result.IsSpam, "score": result.Score}).Debug("Checked mail")
				if result.IsSpam {
					spam = append(spam, m.Uid)
					if !ia.configuration.DryRun {
						if ia.configuration.AppendReports {
							ia.l.WithFields(logrus.Fields{"folder": f, "subject": mail.ShortSubject(m.Subject), "score": result.Score}).Info("Appending spam report")
							err = ia.imapConnection.Put(result.Body, ia.configuration.SpamReportFolder)
							if err != nil {
								return fmt.Errorf(`Could not append report body for "%s" to "%s": %w`, mail.ShortSubject(m.Subject), ia.configuration.SpamReportFolder, err)
							}
						}
					} else {
						ia.l.WithFields(logrus.Fields{"folder": f, "subject": mail.ShortSubject(m.Subject), "score": result.Score}).Info("Not appending report due to dry-run")
					}
				} else {
					// No spam
					ok = append(ok, m.Uid)
				}
			}

			// Move spam mail
			if len(spam) > 0 {
				if !ia.configuration.DryRun {
					if ia.configuration.MoveSpam {
						ia.l.WithFields(logrus.Fields{"folder": f, "spam": len(spam), "destination": ia.configuration.SpamFolder}).Info("Moving spam mails")
						err = ia.imapConnection.Move(spam, ia.configuration.SpamFolder)
						if err != nil {
							return fmt.Errorf(`Could not move spam: %w`, err)
						}
					} else if ia.configuration.DeleteSpam {
						ia.l.WithFields(logrus.Fields{"folder": f, "spam": len(spam)}).Info("Deleting spam mails")
						err = ia.imapConnection.Delete(spam)
						if err != nil {
							return fmt.Errorf(`Could not delete spam: %w`, err)
						}
					}
				} else {
					ia.l.WithFields(logrus.Fields{"folder": f, "spam": len(spam)}).Info("Not moving or deleting spam mails due to dry-run")
				}
			}

			// Only then mark the mails in the database
			saveMails := []domain.SaveMail{}
			for i, m := range mails {
				result := spamResults[i]
				saveMails = append(
					saveMails,
					domain.SaveMail{
						Class:      domain.Checked,
						Uid:        m.Uid,
						MailIdHash: m.MailIdHash,
						FolderName: f,
						Subject:    m.Subject,
						IsSpam:     &result.IsSpam,
						Score:      &result.Score,
					},
				)
			}
			err = ia.persistence.SaveMails(saveMails)
			if err != nil {
				return fmt.Errorf("could not save mails: %w", err)
			}

			totalOk += len(ok)
			totalSpam += len(spam)
			ia.l.WithFields(logrus.Fields{"duration": time.Since(start), "batchsize": len(batch), "ok": len(ok), "spam": len(spam)}).Info("Checked batch")
		}

		err = ia.persistence.SaveFolder(f, uidvalidity)
		if err != nil {
			return fmt.Errorf("could not save uidvalidity for %s: %w", f, err)
		}
	}

	return nil
}

func (ia *ImapAssassin) Learn(learnType domain.LearnType, folders []string) error {
	var class domain.MailClass
	switch learnType {
	case domain.LearnSpam:
		class = domain.LearnedSpam
	case domain.LearnHam:
		class = domain.LearnedHam
	default:
		return fmt.Errorf("unsupported learn type %v", learnType)
	}

	knownFolders, err := ia.persistence.AllFolders()
	if err != nil {
		return fmt.Errorf("could not list known folders: %w", err)
	}

	for _, f := range folders {
		uidvalidity, err := ia.imapConnection.Select(f)
		if err != nil {
			return fmt.Errorf("could not select folder %s: %w", f, err)
		}

		newMailUids, err := ia.getNewMailUids(f, class, knownFolders, uidvalidity)
		if err != nil {
			return fmt.Errorf("could not determine new mail uids: %w", err)
		}

		baseFolderLogger := ia.l.WithFields(logrus.Fields{"folder": f, "learntype": learnType})

		if len(newMailUids) == 0 {
			baseFolderLogger.WithFields(logrus.Fields{"newmails": len(newMailUids)}).Info("Folder contains no new mails to learn")
			break
		}

		if !ia.configuration.DryRun && ia.configuration.DeleteLearned {
			notDeleteReadyReason, err := ia.imapConnection.DeleteReady()
			if err != nil {
				return fmt.Errorf("could not check for delete readiness: %w", err)
			}

			if notDeleteReadyReason != nil {
				ia.l.WithFields(logrus.Fields{"folder": f, "error": notDeleteReadyReason}).Warn("Folder is not ready for mail deletion, skipping")
				continue
			}
		}

		batches := partitionUids(newMailUids, BatchSize)
		baseFolderLogger.WithFields(logrus.Fields{"newmails": len(newMailUids), "batches": len(batches)}).Info("Found mails to learn")

		for _, batch := range batches {
			start := time.Now()
			baseFolderLogger.WithFields(logrus.Fields{"batchsize": len(batch)}).Debug("Learning batch")

			mails, err := ia.imapConnection.FetchMails(batch)
			if err != nil {
				return fmt.Errorf("could not fetch mail batch: %w", err)
			}
			rawMails := make([][]byte, len(mails))
			for i := 0; i < len(mails); i++ {
				rawMails[i] = mails[i].RawMail
			}
			learnResults := ia.spamassassin.LearnAll(learnType, rawMails, LearnConcurrency)

			saveMails := []domain.SaveMail{}
			for i, m := range mails {
				result := learnResults[i]
				if result != nil {
					return fmt.Errorf(`could not learn mail "%s": %w`, mail.ShortSubject(m.Subject), result)
				}
				saveMails = append(
					saveMails,
					domain.SaveMail{
						Class:      class,
						Uid:        m.Uid,
						MailIdHash: m.MailIdHash,
						FolderName: f,
						Subject:    m.Subject,
					},
				)
			}
			err = ia.persistence.SaveMails(saveMails)
			if err != nil {
				return fmt.Errorf("could not save mail: %w", err)
			}

			baseFolderLogger.WithFields(logrus.Fields{"duration": time.Since(start), "batchsize": len(batch)}).Info("Learned batch")

			if ia.configuration.DeleteLearned {
				if ia.configuration.DryRun {
					baseFolderLogger.Info("Not deleting learned mails due to dry-run")
				} else {
					baseFolderLogger.WithFields(logrus.Fields{"batchsize": len(batch)}).Debug("Deleting learned batch")
					err = ia.imapConnection.Delete(batch)
					if err != nil {
						return fmt.Errorf("could not delete batch after learning: %w", err)
					}
					baseFolderLogger.WithFields(logrus.Fields{"duration": time.Since(start), "batchsize": len(batch)}).Info("Deleted learned batch")
				}
			}
		}

		err = ia.persistence.SaveFolder(f, uidvalidity)
		if err != nil {
			return fmt.Errorf("could not save uidvalidity for %s: %w", f, err)
		}

		baseFolderLogger.WithFields(logrus.Fields{"newmails": len(newMailUids), "batches": len(batches)}).Info("Learned mails")
	}

	return nil
}

func (ia *ImapAssassin) getNewMailUids(folder string, class domain.MailClass, knownFolders []*domain.ImapFolder, uidValidity uint32) ([]uint32, error) {
	knownFolder := folderByName(knownFolders, folder)

	newMails, err := ia.imapConnection.ListUids()
	if err != nil {
		return nil, fmt.Errorf("could not list uids in folder: %w", err)
	}
	ia.l.WithFields(logrus.Fields{"folder": folder, "known": knownFolder != nil, "mails": len(newMails)}).Debug("Listed all uids in folder")
	if knownFolder != nil && knownFolder.UidValidity == uidValidity {
		ia.l.WithFields(logrus.Fields{"folder": folder}).Debug("Folder is a known folder and the uid validity hasn't changed, fast uid-based scan is possible")
		knownMails, err := ia.persistence.GetMailsInFolder(class, folder)
		if err != nil {
			return nil, fmt.Errorf("could not list known uids: %w", err)
		}

		for _, m := range knownMails {
			newMails = removeUid(newMails, m.Uid)
		}
	} else if knownFolder != nil && knownFolder.UidValidity != uidValidity {
		ia.l.WithFields(logrus.Fields{"folder": folder}).Debug("Folder is a known folder and but the uid validity has changed, header-based scan is possible")
		mailIds, err := ia.imapConnection.FetchIdHeaders(newMails)
		if err != nil {
			return nil, fmt.Errorf("could not list mail headers for folder: %w", err)
		}

		for _, m := range mailIds {
			knownMail, err := ia.persistence.FindMailByHash(class, folder, m.MailIdHash)
			if err != nil {
				return nil, fmt.Errorf("could not lookup mail via mailIdHash: %w", err)
			}

			if knownMail != nil {
				ia.l.WithFields(logrus.Fields{"folder": folder, "subject": mail.ShortSubject(knownMail.Subject)}).Debug("Is known by hash, updating uid")
				err = ia.persistence.UpdateUid(knownMail.Id, m.Uid)
				if err != nil {
					return nil, fmt.Errorf("could not update uid: %w", err)
				}

				newMails = removeUid(newMails, m.Uid)
			}
		}
	} else {
		ia.l.WithFields(logrus.Fields{"folder": folder}).Debug("Folder is a previously unknown folder, no diff possible")
	}

	sort.Slice(newMails, func(i, j int) bool { return newMails[i] > newMails[j] })
	return newMails, nil
}

func folderByName(knownFolders []*domain.ImapFolder, folder string) *domain.ImapFolder {
	for i := 0; i < len(knownFolders); i++ {
		if knownFolders[i].Name == folder {
			return knownFolders[i]
		}
	}
	return nil
}

func removeUid(newMails []uint32, uid uint32) []uint32 {
	for i := 0; i < len(newMails); i++ {
		if uid == newMails[i] {
			newMails[len(newMails)-1], newMails[i] = newMails[i], newMails[len(newMails)-1]
			newMails = newMails[:len(newMails)-1]
			break
		}
	}
	return newMails
}

// taken from https://github.com/golang/go/wiki/SliceTricks
func partitionUids(uids []uint32, partitionSize int) [][]uint32 {
	batches := make([][]uint32, 0, (len(uids)+partitionSize-1)/partitionSize)

	for partitionSize < len(uids) {
		uids, batches = uids[partitionSize:], append(batches, uids[0:partitionSize:partitionSize])
	}
	batches = append(batches, uids)

	return batches
}

// SPDX-License-Identifier: GPL-3.0-or-later
package imapconnection

import (
	"fmt"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-uidplus"
)

type deleter interface {
	delete([]uint32) error
	deleteReady() (error, error)
}

type uidPlusDeleter struct {
	imapConn      *ImapConnection
	uidplusClient *uidplus.Client
}

func (u *uidPlusDeleter) delete(uids []uint32) error {
	seqset, err := u.imapConn.flagDeleted(uids)
	if err != nil {
		return fmt.Errorf("could not flag items as deleted: %w", err)
	}

	out := make(chan uint32)
	done := make(chan error, 1)
	go func() {
		done <- u.uidplusClient.UidExpunge(seqset, out)
	}()

	expunged := []uint32{}
	for uid := range out {
		expunged = append(expunged, uid)
	}

	err = <-done
	if err != nil {
		return fmt.Errorf("could not expunge mails: %w", err)
	}

	if len(expunged) != len(uids) {
		return fmt.Errorf("unexpected number of expunges, expected %d got %d", len(uids), len(expunged))
	}

	return nil
}

func (u *uidPlusDeleter) deleteReady() (error, error) {
	// UIDPLUS can delete by uid and is therefore always ready
	return nil, nil
}

type compatibilityDeleter struct {
	imapConn *ImapConnection
}

func (c *compatibilityDeleter) delete(uids []uint32) error {
	notDeleteReadyReason, err := c.deleteReady()
	if err != nil {
		return fmt.Errorf("could not check for delete readiness: %w", err)
	}

	if notDeleteReadyReason != nil {
		return fmt.Errorf("folder is not ready for delete: %w", notDeleteReadyReason)
	}

	_, err = c.imapConn.flagDeleted(uids)
	if err != nil {
		return fmt.Errorf("could not set deleted flag: %w", err)
	}

	out := make(chan uint32)
	done := make(chan error, 1)
	go func() {
		done <- c.imapConn.connection.Expunge(out)
	}()

	expunged := []uint32{}
	for uid := range out {
		expunged = append(expunged, uid)
	}

	err = <-done
	if err != nil {
		return fmt.Errorf("could not expunge mails: %w", err)
	}

	if len(expunged) != len(uids) {
		return fmt.Errorf("unexpected number of expunges, expected %d got %d", len(uids), len(expunged))
	}

	return nil
}

var ItemsWithDeletedFlagPresent = fmt.Errorf("folder has previous items with delete flag set")

func (c *compatibilityDeleter) deleteReady() (error, error) {
	// Compatibility read is only ready when there are no mails with deleted flag set.
	// EXPUNGE deletes everything that has the flag set.

	// Get all UIDs in folder with DeletedFlag set
	criteria := imap.NewSearchCriteria()
	criteria.WithFlags = []string{imap.DeletedFlag}
	ids, err := c.imapConn.connection.UidSearch(criteria)
	if err != nil {
		return nil, fmt.Errorf("could search for deleted in folder: %w", err)
	}

	if len(ids) == 0 {
		return nil, nil
	} else {
		return ItemsWithDeletedFlagPresent, nil
	}
}

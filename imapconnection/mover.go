// SPDX-License-Identifier: GPL-3.0-or-later
package imapconnection

import (
	"fmt"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap-move"
)

type mover interface {
	move(uids []uint32, folder string) error
	moveReady() (error, error)
}

type moveMover struct {
	moveClient *move.MoveClient
}

func (m *moveMover) move(uids []uint32, folder string) error {
	seqset := &imap.SeqSet{}
	seqset.AddNum(uids...)
	return m.moveClient.UidMove(seqset, folder)
}

func (m *moveMover) moveReady() (error, error) {
	// MOVE implements move directly and is therefore ready to move all the time
	return nil, nil
}

type compatibilityMover struct {
	imapConn *ImapConnection
}

func (c *compatibilityMover) move(uids []uint32, folder string) error {
	notDeleteReadyReason, err := c.moveReady()
	if err != nil {
		return fmt.Errorf("could not check for delete readiness to move: %w", err)
	}

	if notDeleteReadyReason != nil {
		return fmt.Errorf("folder is not ready for delete, cannot move (copy&delete): %w", notDeleteReadyReason)
	}

	seqset := &imap.SeqSet{}
	seqset.AddNum(uids...)
	err = c.imapConn.connection.UidCopy(seqset, folder)
	if err != nil {
		return fmt.Errorf("could not copy mails: %w", err)
	}

	err = c.imapConn.mailDeleter.delete(uids)
	if err != nil {
		return fmt.Errorf("could not delete copied mails: %w", err)
	}

	return nil
}

func (c *compatibilityMover) moveReady() (error, error) {
	return c.imapConn.mailDeleter.deleteReady()
}

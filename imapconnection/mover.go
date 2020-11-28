// SPDX-License-Identifier: GPL-3.0-or-later
package imapconnection

//go:generate mockgen -destination=mover_mocks_test.go -package=imapconnection -source mover.go
import (
	"fmt"

	"github.com/emersion/go-imap"
)

type moveClient interface {
	UidMove(seqset *imap.SeqSet, dest string) error
}

type moveMover struct {
	moveClient moveClient
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
	imapConn copyAndDeleteMoveClient
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
	err = c.imapConn.UidCopy(seqset, folder)
	if err != nil {
		return fmt.Errorf("could not copy mails: %w", err)
	}

	err = c.imapConn.delete(uids)
	if err != nil {
		return fmt.Errorf("could not delete copied mails: %w", err)
	}

	return nil
}

func (c *compatibilityMover) moveReady() (error, error) {
	return c.imapConn.deleteReady()
}

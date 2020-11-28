// SPDX-License-Identifier: GPL-3.0-or-later
package imapconnection

import "github.com/emersion/go-imap"

//go:generate mockgen -destination=delete_move_mocks_test.go -package=imapconnection -source delete_move.go

// Consolidated file for deleter and mover interfaces used by imapconnection plus the copyAndDeleteMoveClient
// so gomock can generate mocks properly. Unexported interfaces do not allow for reflection mode but source-mode fails
// if there are embedded interfaces spread over multiple source files.

type deleter interface {
	delete([]uint32) error
	deleteReady() (error, error)
}

type mover interface {
	move(uids []uint32, folder string) error
	moveReady() (error, error)
}

type copyAndDeleteMoveClient interface {
	deleter
	UidCopy(seqset *imap.SeqSet, dest string) error
}

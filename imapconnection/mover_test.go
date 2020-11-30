// SPDX-License-Identifier: GPL-3.0-or-later
package imapconnection

import (
	"errors"
	"testing"

	"github.com/emersion/go-imap"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestMoveMover_MoveReady(t *testing.T) {
	mover := moveMover{nil}

	notMoveReadyReason, err := mover.moveReady()
	assert.NoError(t, notMoveReadyReason)
	assert.NoError(t, err)
}

func TestMoveMover_Move(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockmoveClient(ctrl)
	mover := moveMover{conn}

	seqset := &imap.SeqSet{}
	seqset.AddNum(u32a(1, 2, 3)...)
	conn.EXPECT().
		UidMove(gomock.Eq(seqset), gomock.Eq("dest")).
		Return(nil)

	err := mover.move(u32a(1, 2, 3), "dest")
	assert.NoError(t, err)
}

func TestCompatibilityMover_MoveReadyOk(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockcopyAndDeleteMoveClient(ctrl)

	mover := compatibilityMover{conn}

	conn.EXPECT().
		deleteReady().
		Return(nil, nil)

	notMoveReadyReason, err := mover.moveReady()
	assert.NoError(t, notMoveReadyReason)
	assert.NoError(t, err)
}

func TestCompatibilityMover_MoveReadyNotReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockcopyAndDeleteMoveClient(ctrl)

	mover := compatibilityMover{conn}

	notReadyErr := errors.New("delete not ready")
	conn.EXPECT().
		deleteReady().
		Return(notReadyErr, nil)

	notMoveReadyReason, err := mover.moveReady()
	assert.EqualError(t, notMoveReadyReason, notReadyErr.Error())
	assert.NoError(t, err)
}

func TestCompatibilityMover_Move(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockcopyAndDeleteMoveClient(ctrl)

	mover := compatibilityMover{conn}

	conn.EXPECT().
		deleteReady().
		Return(nil, nil)

	seqset := &imap.SeqSet{}
	seqset.AddNum(u32a(1, 2, 3)...)
	conn.EXPECT().
		UidCopy(gomock.Eq(seqset), "dest").
		Return(nil)

	conn.EXPECT().
		delete(u32a(1, 2, 3)).
		Return(nil)

	err := mover.move(u32a(1, 2, 3), "dest")
	assert.NoError(t, err)
}

func TestCompatibilityMover_MoveButNotReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockcopyAndDeleteMoveClient(ctrl)

	mover := compatibilityMover{conn}

	conn.EXPECT().
		deleteReady().
		Return(errors.New("delete not ready"), nil)

	err := mover.move(u32a(1, 2, 3), "dest")
	assert.EqualError(t, err, "folder is not ready for delete, cannot move (copy&delete): delete not ready")
}

// SPDX-License-Identifier: GPL-3.0-or-later
package imapconnection

import (
	"testing"

	"github.com/emersion/go-imap"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestUidPlusDeleter_DeleteReady(t *testing.T) {
	deleter := uidPlusDeleter{nil}

	notDeleteReadyReason, err := deleter.deleteReady()
	assert.NoError(t, notDeleteReadyReason)
	assert.NoError(t, err)
}

func TestUidPlusDeleter_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockdeletedFlaggerAndUidExpunger(ctrl)
	deleter := uidPlusDeleter{conn}

	seqset := &imap.SeqSet{}
	seqset.AddNum(u32a(1, 2, 3)...)
	conn.EXPECT().
		flagDeleted(gomock.Eq(u32a(1, 2, 3))).
		Return(seqset, nil)

	conn.EXPECT().
		UidExpunge(gomock.Eq(seqset), gomock.Any()).
		DoAndReturn(func(seqSet *imap.SeqSet, ch chan uint32) error {
			ch <- u32(1)
			ch <- u32(2)
			ch <- u32(3)
			close(ch)
			return nil
		})

	err := deleter.delete(u32a(1, 2, 3))
	assert.NoError(t, err)
}

func TestCompatibilityDeleter_DeleteReadyOk(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockdeleteFlaggerAndExpunger(ctrl)
	deleter := compatibilityDeleter{conn}

	criteria := imap.NewSearchCriteria()
	criteria.WithFlags = []string{imap.DeletedFlag}

	conn.EXPECT().
		UidSearch(gomock.Eq(criteria)).
		Return(u32a(), nil)

	notDeleteReadyReason, err := deleter.deleteReady()
	assert.NoError(t, notDeleteReadyReason)
	assert.NoError(t, err)
}

func TestCompatibilityDeleter_DeleteReadyNotReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockdeleteFlaggerAndExpunger(ctrl)
	deleter := compatibilityDeleter{conn}

	criteria := imap.NewSearchCriteria()
	criteria.WithFlags = []string{imap.DeletedFlag}

	conn.EXPECT().
		UidSearch(gomock.Eq(criteria)).
		Return(u32a(1), nil)

	notDeleteReadyReason, err := deleter.deleteReady()
	assert.EqualError(t, notDeleteReadyReason, "folder has previous items with delete flag set")
	assert.NoError(t, err)
}

func TestCompatibilityDeleter_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockdeleteFlaggerAndExpunger(ctrl)
	deleter := compatibilityDeleter{conn}

	criteria := imap.NewSearchCriteria()
	criteria.WithFlags = []string{imap.DeletedFlag}

	conn.EXPECT().
		UidSearch(gomock.Eq(criteria)).
		Return(u32a(), nil)

	seqset := &imap.SeqSet{}
	seqset.AddNum(u32a(1, 2, 3)...)
	conn.EXPECT().
		flagDeleted(gomock.Eq(u32a(1, 2, 3))).
		Return(seqset, nil)

	conn.EXPECT().
		Expunge(gomock.Any()).
		DoAndReturn(func(ch chan uint32) error {
			ch <- u32(1)
			ch <- u32(2)
			ch <- u32(3)
			close(ch)
			return nil
		})

	err := deleter.delete(u32a(1, 2, 3))
	assert.NoError(t, err)
}

func TestCompatibilityDeleter_DeleteButNotReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	conn := NewMockdeleteFlaggerAndExpunger(ctrl)
	deleter := compatibilityDeleter{conn}

	criteria := imap.NewSearchCriteria()
	criteria.WithFlags = []string{imap.DeletedFlag}

	conn.EXPECT().
		UidSearch(gomock.Eq(criteria)).
		Return(u32a(1), nil)

	err := deleter.delete(u32a(1, 2, 3))
	assert.EqualError(t, err, "folder is not ready for delete: folder has previous items with delete flag set")
}

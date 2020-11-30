// SPDX-License-Identifier: GPL-3.0-or-later
package imapassassin

import (
	"io/ioutil"
	"testing"

	"github.com/CrawX/go-imap-assassin/domain"
	"github.com/CrawX/go-imap-assassin/domain/mocks"
	"github.com/CrawX/go-imap-assassin/log"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	TEST_FOLDER_1 = "test1"
	TEST_FOLDER_2 = "test2"
)

func setupThreeMails(t *testing.T, cfg *configuration) (*gomock.Controller, *ImapAssassin, *mocks.MockPersistence, *mocks.MockConcurrentSpamClassifier, *mocks.MockImapConnector) {
	ctrl := gomock.NewController(t)

	persistence := mocks.NewMockPersistence(ctrl)
	classifier := mocks.NewMockConcurrentSpamClassifier(ctrl)
	imapConnection := mocks.NewMockImapConnector(ctrl)

	assassin := &ImapAssassin{
		persistence:    persistence,
		imapConnection: imapConnection,
		spamClassifier: classifier,
		configuration:  cfg,
		l:              nullLogger(),
	}

	persistence.EXPECT().
		AllFolders().
		Return(nil, nil)

	imapConnection.EXPECT().
		Select(gomock.Eq(TEST_FOLDER_1)).
		Return(u32(123), nil)

	imapConnection.EXPECT().
		ListUids().
		Return(u32a(1, 2, 3), nil)

	imapConnection.EXPECT().
		FetchMails(gomock.Eq(u32a(3, 2, 1))).
		Return([]*domain.RawImapMail{
			{Uid: 1, RawMail: []byte{1}},
			{Uid: 2, RawMail: []byte{2}},
			{Uid: 3, RawMail: []byte{3}},
		}, nil)

	return ctrl, assassin, persistence, classifier, imapConnection
}

func TestNewImapAssassin(t *testing.T) {
	log.InitLogging("error")
	tests := []struct {
		name string
		cfgs []ConfigFunc
		err  string
	}{
		{"ok", []ConfigFunc{}, ""},
		{"err", []ConfigFunc{MoveSpam("a"), DeleteSpam()}, "error applying configuration: MoveSpam and DeleteSpam cannot be used at the same time"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assassin, err := NewImapAssassin(nil, nil, nil, tc.cfgs...)
			if len(tc.err) == 0 {
				assert.NotNil(t, assassin)
				assert.NoError(t, err)
			} else {
				assert.Nil(t, assassin)
				assert.EqualError(t, err, tc.err)
			}
		})
	}
}

func TestImapAssassin_CheckSpamDryRun(t *testing.T) {
	ctrl, assassin, persistence, classifier, _ := setupThreeMails(t,
		&configuration{
			DryRun:     true,
			DeleteSpam: true,
			MoveSpam:   true,
		},
	)
	defer ctrl.Finish()

	classifier.EXPECT().
		CheckAll(gomock.Eq([][]byte{{1}, {2}, {3}}), gomock.Eq(6)).
		Return([]*domain.SpamResult{{IsSpam: true}, {IsSpam: true}, {IsSpam: true}})

	persistence.EXPECT().
		SaveFolder(TEST_FOLDER_1, u32(123)).
		Return(nil)

	err := assassin.CheckSpam([]string{TEST_FOLDER_1})
	assert.NoError(t, err)
}

func TestImapAssassin_CheckSpamDelete(t *testing.T) {
	ctrl, assassin, persistence, classifier, imapConnection := setupThreeMails(t,
		&configuration{
			DeleteSpam: true,
		},
	)
	defer ctrl.Finish()

	classifier.EXPECT().
		CheckAll(gomock.Eq([][]byte{{1}, {2}, {3}}), gomock.Eq(6)).
		Return([]*domain.SpamResult{{IsSpam: true, Score: 10}, {IsSpam: false}, {IsSpam: true, Score: 10}})

	imapConnection.EXPECT().
		DeleteReady().
		Return(nil, nil)

	imapConnection.EXPECT().
		Delete(gomock.Eq(u32a(1, 3))).
		Return(nil)

	persistence.EXPECT().
		SaveMails(gomock.Any()).
		Do(func(mails []domain.SaveMail) {
			assert.ElementsMatch(t,
				mails,
				[]domain.SaveMail{
					saveMail(domain.Checked, 1, TEST_FOLDER_1, b(true), f(10)),
					saveMail(domain.Checked, 2, TEST_FOLDER_1, b(false), f(0)),
					saveMail(domain.Checked, 3, TEST_FOLDER_1, b(true), f(10)),
				},
			)
		})

	persistence.EXPECT().
		SaveFolder(TEST_FOLDER_1, u32(123)).
		Return(nil)

	err := assassin.CheckSpam([]string{TEST_FOLDER_1})
	assert.NoError(t, err)
}

func TestImapAssassin_CheckSpamMove(t *testing.T) {
	ctrl, assassin, persistence, classifier, imapConnection := setupThreeMails(t,
		&configuration{
			MoveSpam:   true,
			SpamFolder: "spam",
		},
	)
	defer ctrl.Finish()

	classifier.EXPECT().
		CheckAll(gomock.Eq([][]byte{{1}, {2}, {3}}), gomock.Eq(6)).
		Return([]*domain.SpamResult{{IsSpam: true, Score: 10}, {IsSpam: false}, {IsSpam: true, Score: 10}})

	imapConnection.EXPECT().
		MoveReady().
		Return(nil, nil)

	imapConnection.EXPECT().
		Move(gomock.Eq(u32a(1, 3)), gomock.Eq("spam")).
		Return(nil)

	persistence.EXPECT().
		SaveMails(gomock.Any()).
		DoAndReturn(func(mails []domain.SaveMail) error {
			assert.ElementsMatch(t,
				mails,
				[]domain.SaveMail{
					saveMail(domain.Checked, 1, TEST_FOLDER_1, b(true), f(10)),
					saveMail(domain.Checked, 2, TEST_FOLDER_1, b(false), f(0)),
					saveMail(domain.Checked, 3, TEST_FOLDER_1, b(true), f(10)),
				},
			)

			return nil
		})

	persistence.EXPECT().
		SaveFolder(TEST_FOLDER_1, u32(123)).
		Return(nil)

	err := assassin.CheckSpam([]string{TEST_FOLDER_1})
	assert.NoError(t, err)
}

func TestImapAssassin_CheckSpamReport(t *testing.T) {
	ctrl, assassin, persistence, classifier, imapConnection := setupThreeMails(t,
		&configuration{
			AppendReports:    true,
			SpamReportFolder: "reports",
		},
	)
	defer ctrl.Finish()

	classifier.EXPECT().
		CheckAll(gomock.Eq([][]byte{{1}, {2}, {3}}), gomock.Eq(6)).
		Return([]*domain.SpamResult{{IsSpam: true, Score: 10, Body: []byte{0xa}}, {IsSpam: false}, {IsSpam: true, Score: 10, Body: []byte{0xc}}})

	imapConnection.EXPECT().
		Put(gomock.Eq([]byte{0xa}), gomock.Eq("reports")).
		Return(nil)

	imapConnection.EXPECT().
		Put(gomock.Eq([]byte{0xc}), gomock.Eq("reports")).
		Return(nil)

	persistence.EXPECT().
		SaveMails(gomock.Any()).
		DoAndReturn(func(mails []domain.SaveMail) error {
			assert.ElementsMatch(t,
				mails,
				[]domain.SaveMail{
					saveMail(domain.Checked, 1, TEST_FOLDER_1, b(true), f(10)),
					saveMail(domain.Checked, 2, TEST_FOLDER_1, b(false), f(0)),
					saveMail(domain.Checked, 3, TEST_FOLDER_1, b(true), f(10)),
				},
			)

			return nil
		})

	persistence.EXPECT().
		SaveFolder(TEST_FOLDER_1, u32(123)).
		Return(nil)

	err := assassin.CheckSpam([]string{TEST_FOLDER_1})
	assert.NoError(t, err)
}

func TestImapAssassin_LearnDryRun(t *testing.T) {
	for _, learnType := range []domain.LearnType{domain.LearnHam, domain.LearnSpam} {
		t.Run(string(learnType), func(t *testing.T) {
			ctrl, assassin, persistence, classifier, _ := setupThreeMails(t,
				&configuration{
					DryRun: true,
				},
			)
			defer ctrl.Finish()

			classifier.EXPECT().
				LearnAll(learnType, gomock.Eq([][]byte{{1}, {2}, {3}}), gomock.Eq(8)).
				Return([]error{nil, nil, nil})

			persistence.EXPECT().
				SaveFolder(TEST_FOLDER_1, u32(123)).
				Return(nil)

			err := assassin.Learn(learnType, []string{TEST_FOLDER_1})
			assert.NoError(t, err)
		})
	}
}

func TestImapAssassin_LearnNoDelete(t *testing.T) {
	tests := []struct {
		name      string
		learnType domain.LearnType
		mailclass domain.MailClass
	}{
		{string(domain.LearnSpam), domain.LearnSpam, domain.LearnedSpam},
		{string(domain.LearnHam), domain.LearnHam, domain.LearnedHam},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl, assassin, persistence, classifier, _ := setupThreeMails(t,
				&configuration{},
			)
			defer ctrl.Finish()

			classifier.EXPECT().
				LearnAll(tc.learnType, gomock.Eq([][]byte{{1}, {2}, {3}}), gomock.Eq(8)).
				Return([]error{nil, nil, nil})

			persistence.EXPECT().
				SaveMails(gomock.Any()).
				DoAndReturn(func(mails []domain.SaveMail) error {
					assert.ElementsMatch(t,
						mails,
						[]domain.SaveMail{
							saveMail(tc.mailclass, 1, TEST_FOLDER_1, nil, nil),
							saveMail(tc.mailclass, 2, TEST_FOLDER_1, nil, nil),
							saveMail(tc.mailclass, 3, TEST_FOLDER_1, nil, nil),
						},
					)

					return nil
				})

			persistence.EXPECT().
				SaveFolder(TEST_FOLDER_1, u32(123)).
				Return(nil)

			err := assassin.Learn(tc.learnType, []string{TEST_FOLDER_1})
			assert.NoError(t, err)
		})
	}
}

func TestImapAssassin_LearnDelete(t *testing.T) {
	tests := []struct {
		name      string
		learnType domain.LearnType
		mailclass domain.MailClass
	}{
		{string(domain.LearnSpam), domain.LearnSpam, domain.LearnedSpam},
		{string(domain.LearnHam), domain.LearnHam, domain.LearnedHam},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl, assassin, persistence, classifier, imapConnection := setupThreeMails(t,
				&configuration{
					DeleteLearned: true,
				},
			)
			defer ctrl.Finish()

			imapConnection.EXPECT().
				DeleteReady().
				Return(nil, nil)

			classifier.EXPECT().
				LearnAll(tc.learnType, gomock.Eq([][]byte{{1}, {2}, {3}}), gomock.Eq(8)).
				Return([]error{nil, nil, nil})

			imapConnection.EXPECT().
				Delete(u32a(3, 2, 1)).
				Return(nil)

			persistence.EXPECT().
				SaveMails(gomock.Any()).
				DoAndReturn(func(mails []domain.SaveMail) error {
					assert.ElementsMatch(t,
						mails,
						[]domain.SaveMail{
							saveMail(tc.mailclass, 1, TEST_FOLDER_1, nil, nil),
							saveMail(tc.mailclass, 2, TEST_FOLDER_1, nil, nil),
							saveMail(tc.mailclass, 3, TEST_FOLDER_1, nil, nil),
						},
					)

					return nil
				})

			persistence.EXPECT().
				SaveFolder(TEST_FOLDER_1, u32(123)).
				Return(nil)

			err := assassin.Learn(tc.learnType, []string{TEST_FOLDER_1})
			assert.NoError(t, err)
		})
	}
}

func TestImapAssassin_getNewMailUids(t *testing.T) {
	tests := []struct {
		name string

		folder       string
		knownFolders []*domain.ImapFolder
		uidValidity  uint32

		imapUids []uint32

		knownUids []uint32

		idHeaders   map[string]uint32
		knownHashes []string

		expectedNew []uint32
	}{
		{
			"unknownfolder",
			TEST_FOLDER_1, imapFolder(TEST_FOLDER_2, 123), 123,
			u32a(1, 2),
			nil,
			nil, nil,
			u32a(1, 2),
		},
		{
			"knownfolder_uidvalidity_unchanged",
			TEST_FOLDER_1, imapFolder(TEST_FOLDER_1, 123), 123,
			u32a(1, 2, 3),
			u32a(1, 3),
			nil, nil,
			u32a(2),
		},
		{
			"knownfolder_uidvalidity_changed",
			TEST_FOLDER_1, imapFolder(TEST_FOLDER_1, 123), 124,
			u32a(1, 2, 3),
			nil,
			map[string]uint32{"a": 1, "b": 2, "c": 3}, []string{"a", "c"},
			u32a(2),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			persistence := mocks.NewMockPersistence(ctrl)
			imapConnection := mocks.NewMockImapConnector(ctrl)

			assassin := &ImapAssassin{
				persistence:    persistence,
				imapConnection: imapConnection,
				l:              nullLogger(),
			}

			imapConnection.EXPECT().ListUids().Return(tc.imapUids, nil)

			// known & uidvalidity unchanged
			if tc.knownUids != nil {
				stubMails := []*domain.SavedImapMail{}
				for _, uid := range tc.knownUids {
					stubMails = append(stubMails, &domain.SavedImapMail{Uid: uid})
				}
				persistence.EXPECT().GetMailsInFolder(gomock.Eq(domain.Checked), gomock.Eq(TEST_FOLDER_1)).Return(stubMails, nil)
			}

			// known & uidvalidity has changed
			if tc.idHeaders != nil {
				stubMails := []*domain.ImapIdInfo{}
				for hash, uid := range tc.idHeaders {
					stubMails = append(stubMails, &domain.ImapIdInfo{Uid: uid, MailIdHash: hash})

					known := -1
					for i := 0; i < len(tc.knownHashes); i++ {
						if hash == tc.knownHashes[i] {
							known = i
							break
						}
					}

					if known > -1 {
						persistence.EXPECT().FindMailByHash(gomock.Eq(domain.Checked), gomock.Eq(tc.folder), gomock.Eq(hash)).
							Return(&domain.SavedImapMail{Id: int64(known)}, nil)
						persistence.EXPECT().UpdateUid(gomock.Eq(int64(known)), gomock.Eq(uid))
					} else {
						persistence.EXPECT().FindMailByHash(gomock.Eq(domain.Checked), gomock.Eq(tc.folder), gomock.Eq(hash)).
							Return(nil, nil)
					}
				}
				imapConnection.EXPECT().FetchIdHeaders(gomock.Eq(tc.imapUids)).Return(stubMails, nil)
			}

			uids, err := assassin.getNewMailUids(tc.folder, domain.Checked, tc.knownFolders, tc.uidValidity)
			assert.NoError(t, err)
			assert.ElementsMatch(t, tc.expectedNew, uids)
		})
	}
}

func Test_partitionUids(t *testing.T) {
	tests := []struct {
		name     string
		input    []uint32
		expected [][]uint32
	}{
		{"singlepartition", u32a(1), [][]uint32{u32a(1)}},
		{"multiple", u32a(1, 2, 3, 4, 5), [][]uint32{u32a(1, 2), u32a(3, 4), u32a(5)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uids := partitionUids(tc.input, 2)
			assert.Equal(t, tc.expected, uids)
		})
	}
}

func nullLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)
	return logger
}

func u32(val int) uint32 {
	return uint32(val)
}

func u32a(val ...int) []uint32 {
	a := []uint32{}
	for _, v := range val {
		a = append(a, u32(v))
	}

	return a
}

func b(val bool) *bool {
	return &val
}

func f(val float64) *float64 {
	return &val
}

func saveMail(class domain.MailClass, uid uint32, folderName string, isSpam *bool, score *float64) domain.SaveMail {
	return domain.SaveMail{
		Class:      class,
		Uid:        uid,
		MailIdHash: "",
		FolderName: folderName,
		Subject:    "",
		IsSpam:     isSpam,
		Score:      score,
	}
}

func imapFolder(name string, uidValidity int) []*domain.ImapFolder {
	return []*domain.ImapFolder{{
		Name:        name,
		UidValidity: u32(uidValidity),
	}}
}

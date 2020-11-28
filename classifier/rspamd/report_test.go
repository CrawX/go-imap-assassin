// SPDX-License-Identifier: GPL-3.0-or-later
package rspamd

import (
	"bytes"
	stdmail "net/mail"
	"testing"

	"github.com/stretchr/testify/assert"
)

const MAIL = `Return-Path: <bounce@chefkoch.de>
Received: from ddx.blubb.xyz ([123.123.123.123]) by mx.emig.kundenserver.de
 (mxeue010 [123.123.123.123]) with ESMTP (Nemesis) id 1MpmLV-1kiiMq2THx-00qEYb
 for <crawx@crawx.crawx>; Wed, 07 Oct 2020 01:30:45 +0200
Date: mon, 7 Feb 2106 00:19:19 +0100
To: someone@online.de
From: Ebike<shop@lidl.de>
Subject: Gewinen ein Ebike
Message-ID: <a653c0356ab3250a87fb358c631962ed@localhost.localdomain>
X-Priority: 3
X-Mailer: PHPMailer 5.2.7 (https://github.com/PHPMailer/PHPMailer/)
MIME-Version: 1.0
Content-Type: multipart/alternative;
	boundary="b1_a653c0356ab3250a87fb358c631962ed"
Content-Transfer-Encoding: 8bit
X-Spam-Flag: NO
Envelope-To: <crawx@crawx.crawx>
X-Spam-Flag: NO

Testmail`

func Test_report(t *testing.T) {
	r, err := report([]byte(MAIL), []byte("{resp}"), 3.0)

	assert.NoError(t, err)
	assert.Contains(t, string(r), MAIL)
	assert.Contains(t, string(r), "{resp}")

	msg, err := stdmail.ReadMessage(bytes.NewReader(r))
	assert.NoError(t, err, "report should be parsable")

	assert.Len(t, msg.Header["X-Spam-Status"], 1)
	assert.Equal(t, "X-Spam-Status: Yes, score=3.0", msg.Header["X-Spam-Status"][0])
	assert.Len(t, msg.Header["X-Spam-Flag"], 1)
	assert.Equal(t, "***", msg.Header["X-Spam-Flag"][0])
	assert.Len(t, msg.Header["X-Spam-Checker-Version"], 1)
	assert.Equal(t, "rspamd", msg.Header["X-Spam-Checker-Version"][0])
}

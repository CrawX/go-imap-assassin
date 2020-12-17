-- SPDX-License-Identifier: GPL-3.0-or-later

-- +migrate Up

-- +migrate StatementBegin
drop index messages_class_foldername_mailidhash_index;

create index messages_mailidhash_class_foldername_index
	on messages (mailidhash, class, foldername);

-- +migrate StatementEnd

-- SPDX-License-Identifier: GPL-3.0-or-later

-- +migrate Up

-- +migrate StatementBegin
create table folders
(
	name            string
		            primary key,
	uidvalidity     integer
	                not null
);

create table messages
(
	id              integer
			        primary key autoincrement,
	class           integer
	                not null,
	uid             integer
	                not null,
	mailidhash      string
	                not null,
	foldername      string
	                not null,
	subject         string
	                not null,
    isspam          bool,
    score           real
);

create index messages_class_foldername_mailidhash_index
	on messages (class, foldername, mailidhash);

-- +migrate StatementEnd

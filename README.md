# go-imap-assassin
go-imap-assassin is a command-line application that leverages the popular spam classifiers SpamAssassin or Rspamd to remove spam in IMAP mailboxes.

## Motivation
Most mailbox providers do some form of spam-checking for clients. The quality of the filter heavily depends on the specific provider, 
and the providers offer different levels of control over the filter, such as improving the personal classifier by "learning" known
bad (spam) or good (ham) mails.

If the chosen mail providers' spam classification doesn't work well enough, short of switching mailboxes (which usually means also switching addresses),
the client is usually bound by the level of control the provider offers.

This is where `go-imap-assassin` comes into play: it uses the popular [Spamassassin](https://spamassassin.apache.org) or [Rspamd](https://rspamd.com) anti-spam platform
to classify mail that is already in the client's mailbox by retrieving it via IMAP. According to the classification done by `SpamAssassin` or `Rspamd`, 
the respective spam mails can be moved, deleted etc. by `go-imap-assassin`.

This implementation uses the same concept implemented in [isbg](https://gitlab.com/isbg/isbg). However, this is a full re-implementation that leverages
[Go](https://golang.org)'s very rich standard library & concurrency features. See below for a comparison with `isbg`. I decided to create this
re-write because my mailbox contained mails that Python3's mail implementation [couldn't process](https://gitlab.com/isbg/isbg/-/issues/152) and after
a couple of hours of trying to solve it in Python3, I was unable to get it done. Go's standard library has no problems with the mails I tried.

## Features
* Broad IMAP compatibility, making use of IMAP extensions when available 
* Efficient handling of IMAP specifics, such as `UIDVALIDITY` changes
* Robust mail parsing via Go's standard library
* Concurrent access to `SpamAssassin` or `Rspamd` to improve classification throughput
* Stores mail UIDs plus metadata in a standard `sqlite` database

## Development progress
Although the core functionality is implemented and I'm slowly starting to use this on my personal mailbox, this is not a finished product.

I'm working on this in my spare time and will address the remaining to-dos as time permits. The following should give an approximate overview:
- [x] Implement IMAP & SpamAssassin access
- [x] Implement Rspamd as a classifier
- [x] Implement mail filter & delete/move
- [x] Implement filter training
- [x] Implement compatibility for IMAP servers without `MOVE` or `UIDPLUS`
- [x] Do some basic testing with multiple imap servers
- [ ] Add unit tests
- [x] Add basic CI/CD chain
- [ ] Add multiplatform release CI/CD using xgo
- [ ] Document build steps
- [ ] Document application code
- [ ] Document setup steps (including `SpamAssassin` and `Rspamd` setup)
- [ ] Document configuration
- [ ] Add continuous running mode using `IMAP-IDLE`
- [ ] Broader tests with more IMAP servers

## Comparison with `isbg`
See below for a comparison with [isbg](https://gitlab.com/isbg/isbg). Most information is taken from the Gitlab page, some are my
interpretations of the source code.

| Feature                       | `go-imap-assassin`                                                        | `isbg`                                                                                        |
| -------------                 |:-------------                                                             | :-----                                                                                        |
| Programming language          | Go                                                                        | Python3                                                                                       |
| Classifier support            | `SpamAssassin`, `Rspamd`                                                  | `SpamAssassin`                                                                                |
| Classifiers access            | concurrent                                                                | single-threaded                                                                               |
| Already-processed detection   | UID- and header-based, fast diff mechanism, `sqlite` storage              | UID-based, `UIDVALIDITY` change will trigger rescan, `json` file storage                      |
| Configuration                 | file-based, [toml](https://github.com/toml-lang/toml) configuration       | commandline-parameter based configuration                                                     |
| Maturity                      | Work in progress                                                          | Stable with considerable userbase                                                             |
| Tests                         | Work in progress                                                          | Unit tests                                                                                    |
| Maintainership & community    | Maintained and used by @CrawX                                             | Maintained by multiple people, mostly by @baldurmen, community which reports and fixes bugs   |


## Acknowledgements
* [isbg](https://gitlab.com/isbg/isbg) for an alternative and more mature implementation of the same concept
* [go-imap](https://github.com/emersion/go-imap) for a fully fledged IMAP library
* [sqlite](https://www.sqlite.org) for being a fantastic embeddable database and [go-sqlite3](https://github.com/mattn/go-sqlite3) for allow easy integration with Go
* [SpamAssassin](https://spamassassin.apache.org) for offering a very good classifier thatn can easily outperform the classifier offered by my mailbox provider
* [Rspamd](https://rspamd.com) for offering a more modern and more efficient alternative to SpamAssassin
* [spamc](https://github.com/teamwork/spamc) for offering a Go-native way to access spamassassin
* Various other libraries I used such as [toml](https://github.com/BurntSushi/toml), [sqlx](https://github.com/jmoiron/sqlx) or [logrus](https://github.com/sirupsen/logrus)

## License
This application is licensed under [GPLv3](https://www.gnu.org/licenses/gpl-3.0.en.html). There is no warranty, to the extent permitted by law.
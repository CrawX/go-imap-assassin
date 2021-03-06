// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"flag"
	"strings"

	"github.com/CrawX/go-imap-assassin/classifier"
	"github.com/CrawX/go-imap-assassin/classifier/rspamd"
	"github.com/CrawX/go-imap-assassin/classifier/spamassassin"
	"github.com/CrawX/go-imap-assassin/config"
	"github.com/CrawX/go-imap-assassin/domain"
	"github.com/CrawX/go-imap-assassin/imapassassin"
	"github.com/CrawX/go-imap-assassin/imapconnection"
	"github.com/CrawX/go-imap-assassin/log"
	"github.com/CrawX/go-imap-assassin/persistence"

	"github.com/sirupsen/logrus"
)

func main() {
	log.InitLogging("debug")
	logger := log.Logger(log.LOG_MAIN)

	configFile := flag.String("config", "config.toml", "config file to load")
	flag.Parse()

	conf, err := config.ReadConfig(*configFile)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "configfile": *configFile}).Fatal("Could not load config")
	}

	if conf.Loglevel != nil {
		log.SetLogLevel(*conf.Loglevel)
	}

	p, err := persistence.NewPersistence(conf.Database)
	if err != nil {
		logger.WithField("error", err).Fatal("Could not connect to database")
	}
	defer p.Close()

	var spamClassifier domain.SpamClassifier
	if conf.SpamassassinHost != "" {
		logger.WithFields(logrus.Fields{"classifier": "spamassassin", "spamassssinhost": conf.SpamassassinHost}).Info("Using SpamAssassin")
		spamClassifier, err = spamassassin.NewSpamassassin(conf.SpamassassinHost)
		if err != nil {
			logger.WithField("error", err).Fatal("Could not start SpamAssassin connector")
		}
	} else {
		controllerWithoutTrailingSlashes := strings.TrimRight(conf.RspamdController, "/")
		logger.WithFields(logrus.Fields{"classifier": "rspamd", "rspamdcontroller": controllerWithoutTrailingSlashes}).Info("Using Rspamd")
		spamClassifier, err = rspamd.NewRspamd(controllerWithoutTrailingSlashes, conf.RspamdPassword)
		if err != nil {
			logger.WithField("error", err).Fatal("Could not start rspamd connector")
		}
	}

	imapConn, err := imapconnection.NewImapConnection(conf.ImapHost, conf.User, conf.Password)
	if err != nil {
		logger.WithField("error", err).Fatal("Could not start imap connector")
	}
	defer imapConn.Close()

	configs := []imapassassin.ConfigFunc{}
	if conf.DryRun {
		configs = append(configs, imapassassin.DryRun())
	}

	if conf.DeleteSpam {
		configs = append(configs, imapassassin.DeleteSpam())
	}
	if conf.MoveSpam {
		configs = append(configs, imapassassin.MoveSpam(conf.SpamFolder))
	}
	if conf.AppendReports {
		configs = append(configs, imapassassin.AppendReports(conf.ReportFolder))
	}

	if conf.DeleteLearned {
		configs = append(configs, imapassassin.DeleteLearned())
	}

	sc, err := imapassassin.NewImapAssassin(p, &classifier.GoRoutineSpamClassifier{SpamClassifier: spamClassifier}, imapConn, configs...)
	if err != nil {
		logger.WithField("error", err).Fatal("Could not start spamchecker")
	}

	if len(conf.SpamLearnFolders) > 0 || len(conf.HamLearnFolders) > 0 {
		logger.WithFields(logrus.Fields{"spamfolders": conf.SpamLearnFolders, "hamfolders": conf.HamLearnFolders, "deletelearned": conf.DeleteLearned, "dryrun": conf.DryRun}).Info("Learning mails")
		if conf.DeleteLearned {
			if conf.DryRun {
				logger.Warn("Skipping deletion of learned mails due to dry-run")
			} else {
				logger.Info("Learned mails will be deleted from server afterwards")
			}
		} else {
			logger.Info("Not deleting mails after learning them")
		}

		if len(conf.SpamLearnFolders) > 0 {
			err = sc.Learn(domain.LearnSpam, conf.SpamLearnFolders)
			if err != nil {
				logger.WithField("error", err).Fatal("Learning spam failed")
			}
		}

		if len(conf.HamLearnFolders) > 0 {
			err = sc.Learn(domain.LearnHam, conf.HamLearnFolders)
			if err != nil {
				logger.WithField("error", err).Fatal("Learning spam failed")
			}
		}
	}

	logger.WithFields(logrus.Fields{"folders": conf.CheckFolders, "dryrun": conf.DryRun, "spamfolder": conf.SpamFolder}).Info("Checking mails for spam")
	if conf.DryRun {
		logger.Warn("Skipping moving & report generation due to dry-run")
	}
	err = sc.CheckSpam(conf.CheckFolders)
	if err != nil {
		logger.WithField("error", err).Fatal("Checking spam failed")
	}
}

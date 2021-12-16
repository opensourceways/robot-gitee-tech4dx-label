package main

import (
	"fmt"

	"github.com/opensourceways/community-robot-lib/config"
	framework "github.com/opensourceways/community-robot-lib/robot-gitee-framework"
	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
)

const botName = "tech4dx-label"

type iClient interface {
	CreateIssueComment(org, repo string, number string, comment string) error
	AddMultiIssueLabel(org, repo, number string, label []string) error
	GetRepoLabels(owner, repo string) ([]sdk.Label, error)
	AssignGiteeIssue(org, repo string, number string, login string) error
}

func newRobot(cli iClient) *robot {
	return &robot{cli: cli}
}

type robot struct {
	cli iClient
}

func (bot *robot) NewConfig() config.Config {
	return &configuration{}
}

func (bot *robot) getConfig(cfg config.Config, org, repo string) (*botConfig, error) {
	c, ok := cfg.(*configuration)
	if !ok {
		return nil, fmt.Errorf("can't convert to configuration")
	}

	if bc := c.configFor(org, repo); bc != nil {
		return bc, nil
	}

	return nil, fmt.Errorf("no config for this repo:%s/%s", org, repo)
}

func (bot *robot) RegisterEventHandler(f framework.HandlerRegitster) {
	f.RegisterIssueHandler(bot.handleIssueEvent)
	f.RegisterNoteEventHandler(bot.handleNoteEvent)
}

func (bot *robot) handleIssueEvent(e *sdk.IssueEvent, c config.Config, log *logrus.Entry) error {
	org, repo := e.GetOrgRepo()

	cfg, err := bot.getConfig(c, org, repo)
	if err != nil {
		return err
	}

	return bot.handleIssueCreate(e, cfg, log)
}

func (bot *robot) handleNoteEvent(e *sdk.NoteEvent, c config.Config, log *logrus.Entry) error {
	return nil
}

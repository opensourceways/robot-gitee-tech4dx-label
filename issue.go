package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/opensourceways/community-robot-lib/giteeclient"
	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	wordsRegStr = "(mailinglist|maillist|mail|邮件|邮箱|subscribe|订阅" +
		"|etherpad|meetingrecord|会议记录" +
		"|cla|CLA|signagreement|签署贡献者协议" +
		"|guarding|jenkins|staticcheck|test|compile|robot|测试|编译|检查" +
		"|website|blog|mirror|下载|官网|博客|镜像" +
		"|meeting|会议|例会" +
		"|sensitivewords|敏感词" +
		"|log|日志" +
		"|docs|documents|文档" +
		"|labelsetting|标签设置" +
		"|access|permission|权限" +
		"|requirement|featurerequest|需求" +
		"|translation|翻译" +
		"|bug|BUG|cve|CVE" +
		"|gitee|Gitee|Git|git" +
		"|scheduling|调度" +
		"|obs|OBS|rpm|PRM|iso|ISO" +
		"|src-openeuler|src-openEuler|openeuler|openEuler)" +
		"|开源实习"

	labelRegStr = `(?m)//(mailing|etherpad|CLA|guarding|website|meeting|kind|bug|CVE|security|activity|gitee|git|sig|release|build|repo)(\S*)`
)

const (
	bugType               = "Bug"
	requireType           = "Requirement"
	cveType               = "CVE和安全问题"
	translateType         = "翻译"
	openSoucePracticeType = "开源实习"
	taskType              = "Task"
)

var (
	wordsReg = regexp.MustCompile(wordsRegStr)
	labelReg = regexp.MustCompile(labelRegStr)
)

type Mentor struct {
	Words []string `json:"words"`
	Label string   `json:"label"`
	Name  string   `json:"name"`
}

func (bot *robot) handleIssueCreate(e *sdk.IssueEvent, cfg *botConfig, log *logrus.Entry) error {
	if e.GetAction() != giteeclient.StatusOpen {
		return nil
	}

	org, repo := e.GetOrgRepo()
	number := e.GetIssueNumber()

	if err := bot.createIssueCommentByTpl(cfg.IssueCommetTplPath, org, repo, number); err != nil {
		return err
	}

	if len(e.GetIssueLabelSet()) == 0 {
		return bot.handleIssueNoLabels(e, cfg, log)
	}

	return bot.handleIssueHasLabels(e, cfg, log)
}

func (bot *robot) handleIssueNoLabels(e *sdk.IssueEvent, cfg *botConfig, log *logrus.Entry) error {
	issue := e.GetIssue()
	org, repo := e.GetOrgRepo()
	labels := parseLabelsFromIssue(issue.GetBody(), issue.GetTypeName())

	repoLabels, err := bot.getRepoLabelSet(org, repo)
	if err != nil {
		return err
	}

	canAdd := labels.Intersection(repoLabels)

	if err := bot.cli.AddMultiIssueLabel(org, repo, issue.GetNumber(), canAdd.UnsortedList()); err != nil {
		return err
	}

	if issue.GetAssignee().GetLogin() == "" {
		assignee, err := getLabelAssignee(cfg.MenterPath, labels)
		if err != nil {
			return err
		}

		if err := bot.cli.AssignGiteeIssue(org, repo, issue.GetNumber(), assignee); err != nil {
			return err
		}
	}

	return bot.handleBodyWords(e, cfg, log)
}

func (bot *robot) handleBodyWords(e *sdk.IssueEvent, cfg *botConfig, log *logrus.Entry) error {

	return nil
}

func (bot *robot) handleIssueHasLabels(e *sdk.IssueEvent, cfg *botConfig, log *logrus.Entry) error {
	return nil
}

func (bot *robot) createIssueCommentByTpl(tplPath, org, repo, numer string) error {
	comment, err := loadPathContent(tplPath)
	if err != nil {
		return err
	}

	return bot.cli.CreateIssueComment(org, repo, numer, string(comment))
}

func (bot *robot) getRepoLabelSet(org, repo string) (sets.String, error) {
	repoLabels := sets.NewString()

	rl, err := bot.cli.GetRepoLabels(org, repo)
	if err != nil {
		return repoLabels, err
	}

	for _, v := range rl {
		repoLabels.Insert(v.Name)
	}

	return repoLabels, nil
}

func parseLabelsFromIssue(body, issueType string) sets.String {
	labels := sets.NewString()

	matches := labelReg.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		l := strings.TrimSpace(strings.TrimLeft(m[0], "//"))
		ws := strings.Split(l, ",")

		for _, w := range ws {
			labels.Insert(w)
		}
	}

	switch issueType {
	case bugType:
		labels.Insert("bug/unconfirmed")
	case requireType:
		labels.Insert("kind/feature_request")
	case cveType:
		labels.Insert("cve/pending")
	case translateType:
		labels.Insert("kind/translation")
	case openSoucePracticeType:
		labels.Insert("activity/开源实习")
	default:
		labels.Insert("kind/task")
	}

	return labels
}

func parseWordsFromIssue(body string) sets.String {
	ibody := strings.ReplaceAll(body, " ", "")
	ibody = strings.ReplaceAll(ibody, "\n", "")
	words := sets.NewString()

	matcheWords := wordsReg.FindAllStringSubmatch(ibody, -1)
	if len(matcheWords) == 0 {
		return words
	}

	for _, match := range matcheWords {
		w := strings.ToLower(strings.TrimSpace(match[0]))
		words.Insert(w)
	}

	return words
}

func (bot *robot) genIssueAssignee(issueAuthor string) (string, error) {
	orgOrigin := "open_euler"

	isEntUser := func() bool {
		return issueAuthor == orgOrigin
	}

	if isEntUser() {
		return issueAuthor, nil
	}

}

func getLabelAssignee(mentorPath string, labels sets.String) (string, error) {
	v, err := loadPathContent(mentorPath)
	if err != nil {
		return "", err
	}

	var mentors []Mentor

	if err := json.Unmarshal(v, &mentors); err != nil {
		return "", err
	}

	for _, v := range mentors {
		if labels.Has(v.Label) {
			return v.Name, nil
		}
	}

	return "", nil
}

func loadPathContent(path string) ([]byte, error) {
	v, err := ioutil.ReadFile(path)
	if err != nil {
		return v, fmt.Errorf("read %s template file failed: %s", path, err)
	}

	return v, nil
}

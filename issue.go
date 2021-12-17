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
	imPlacehold           = "{issueMaker}"
	asPlacehold           = "{assignee}"
	gfi                   = "good-first-issue"
	decisLable            = "kind/decision"
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

func (bot *robot) handleIssueComment(e *sdk.NoteEvent, cfg *botConfig, log *logrus.Entry) error {
	if !e.IsCreatingCommentEvent() {
		return nil
	}

	if yes, err := bot.isBotComment(e.GetCommenter()); err != nil || yes {
		return err
	}

	noteBody := e.GetComment().GetBody()

	labels := parseLabelsFromMatch(noteBody)
	if len(labels) == 0 {
		return nil
	}

	org, repo := e.GetOrgRepo()
	number := e.GetIssueNumber()

	if err := bot.addMutilLabels(org, repo, number, labels); err != nil {
		return err
	}

	if strings.Contains(noteBody, gfi) {
		comment := "如果您是第一次贡献社区，可以参考我们的贡献指南：https://www.openeuler.org/zh/community/contribution/"
		if err := bot.cli.CreateIssueComment(org, repo, number, comment); err != nil {
			return err
		}
	}

	assignee := e.GetIssue().GetAssignee().GetLogin()
	if assignee == "" {
		assignee, err := getLabelAssignee(cfg.MenterPath, labels)
		if err != nil {
			return err
		}

		if err := bot.cli.AssignGiteeIssue(org, repo, number, assignee); err != nil {
			return err
		}
	}

	return bot.handleLabelsHasDesision(labels, org, repo, number, assignee, e.GetIssueAuthor(), cfg.DescisionTplPath)
}

func (bot *robot) handleIssueNoLabels(e *sdk.IssueEvent, cfg *botConfig, log *logrus.Entry) error {
	issue := e.GetIssue()
	org, repo := e.GetOrgRepo()
	number := e.GetIssueNumber()
	labels := parseLabelsFromIssue(issue.GetBody(), issue.GetTypeName())

	if err := bot.addMutilLabels(org, repo, number, labels); err != nil {
		return err
	}

	assignee := issue.GetAssignee().GetLogin()
	if assignee == "" {
		assignee, err := bot.genIssueAssignee(cfg.MenterPath, e.GetIssueAuthor(), labels)
		if err != nil {
			return err
		}

		if err := bot.cli.AssignGiteeIssue(org, repo, issue.GetNumber(), assignee); err != nil {
			return err
		}
	}

	return bot.handleIssueBodyWords(assignee, labels, e, cfg)
}

func (bot *robot) addMutilLabels(org, repo, number string, labels sets.String) error {
	repoLabels, err := bot.getRepoLabelSet(org, repo)
	if err != nil {
		return err
	}

	canAdd := labels.Intersection(repoLabels)

	return bot.cli.AddMultiIssueLabel(org, repo, number, canAdd.UnsortedList())
}

func (bot *robot) handleIssueHasLabels(e *sdk.IssueEvent, cfg *botConfig, log *logrus.Entry) error {
	org, repo := e.GetOrgRepo()
	number := e.GetIssueNumber()
	author := e.GetIssueAuthor()

	assingnee := e.GetIssue().GetAssignee().GetLogin()
	if assingnee == "" && bot.isEntUser(author) {
		if err := bot.cli.AssignGiteeIssue(org, repo, number, assingnee); err != nil {
			return err
		}
	}

	c, err := loadPathContent(cfg.AssignTplPath)
	if err != nil {
		return err
	}

	labels := e.GetIssueLabelSet()
	assingnee, err = getLabelAssignee(cfg.MenterPath, labels)
	if err != nil {
		return err
	}

	atpl := strings.ReplaceAll(string(c), imPlacehold, author)

	if assingnee != "" {
		if assingnee != author {
			atpl = strings.ReplaceAll(atpl, asPlacehold, assingnee)
		} else {
			self := "自己"
			atpl = strings.ReplaceAll(atpl, "@"+assingnee, self)
		}

		if err := bot.cli.CreateIssueComment(org, repo, number, atpl); err != nil {
			return err
		}
	}

	return bot.handleLabelsHasDesision(labels, org, repo, number, assingnee, author, cfg.DescisionTplPath)
}

func (bot *robot) handleLabelsHasDesision(
	labels sets.String,
	org, repo, number, assignee, author, dpath string,
) error {
	if !labels.Has(decisLable) {
		return nil
	}

	d, err := loadPathContent(dpath)
	if err != nil {
		return err
	}

	str := ""
	if assignee != "" && assignee != author {
		str = fmt.Sprintf(" @%s", assignee)
	}

	return bot.createIssueCommentByTpl(
		org, repo, number,
		fmt.Sprintf("hello, @%s%s%s\n", author, str, string(d)),
	)
}

func (bot *robot) handleIssueBodyWords(
	assignee string,
	labesToAdd sets.String,
	e *sdk.IssueEvent,
	cfg *botConfig,
) error {
	words := parseWordsFromIssue(e.GetIssue().GetBody())
	if len(words) == 0 {
		return nil
	}

	labels, err := getLabelsFromWords(words, cfg.MenterPath)
	if err != nil {
		return err
	}

	if len(labels) == 0 {
		return nil
	}

	labelsFind := labels.Difference(labesToAdd)
	if len(labelsFind) == 0 {
		return nil
	}

	return bot.createIssueHelloComment(cfg.MenterPath, cfg.ParitiTplPath, assignee, labelsFind, e)
}

func (bot *robot) createIssueHelloComment(
	mentorPath, partiPath, assignee string,
	labes sets.String,
	e *sdk.IssueEvent,
) error {
	lstr := ""
	for _, v := range labes.UnsortedList() {
		lstr = lstr + fmt.Sprintf("**//%s**\n", v)
	}

	ptpl, err := loadPathContent(partiPath)
	if err != nil {
		return err
	}

	giPlacehold := "{goodissue}"
	lbPlacehold := "{label}"
	gfiDesc := "因为这个issue看起来是文档类问题, 适合新手开发者解决"
	author := e.GetIssueAuthor()
	hellComment := strings.ReplaceAll(string(ptpl), imPlacehold, author)

	if assignee != "" && assignee != author {
		hellComment = strings.ReplaceAll(hellComment, asPlacehold, assignee)
	} else {
		hellComment = strings.ReplaceAll(hellComment, "@"+asPlacehold, "")
	}

	if strings.Contains(lstr, gfi) {
		hellComment = strings.ReplaceAll(hellComment, giPlacehold, gfiDesc)
	} else {
		hellComment = strings.ReplaceAll(hellComment, giPlacehold, "")
	}

	hellComment = strings.ReplaceAll(hellComment, lbPlacehold, lstr)
	org, repo := e.GetOrgRepo()
	number := e.GetIssueNumber()

	return bot.cli.CreateIssueComment(org, repo, number, hellComment)
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

func (bot *robot) genIssueAssignee(mentorPath, issueAuthor string, labes sets.String) (string, error) {
	if bot.isEntUser(issueAuthor) {
		return issueAuthor, nil
	}

	return getLabelAssignee(mentorPath, labes)
}

func (bot *robot) isEntUser(login string) bool {
	orgOrigin := "open_euler"

	// TODO:  判断作者是不是企业成员
	return login == orgOrigin

}

func (bot *robot) isBotComment(login string) (bool, error) {
	b, err := bot.cli.GetBot()
	if err != nil {
		return false, err
	}

	return b.Login == login, nil
}

func getLabelsFromWords(words sets.String, path string) (sets.String, error) {
	metors, err := getMentors(path)
	if err != nil {
		return nil, err
	}

	labels := sets.NewString()

	for k := range words {
		for _, v := range metors {
			mWords := sets.NewString(v.Words...)
			if mWords.Has(k) {
				labels.Insert(v.Label)
				break
			}
		}

	}

	return labels, nil
}

func parseLabelsFromIssue(body, issueType string) sets.String {
	labels := parseLabelsFromMatch(body)

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

func parseLabelsFromMatch(mstr string) sets.String {
	labels := sets.NewString()

	matches := labelReg.FindAllStringSubmatch(mstr, -1)
	for _, m := range matches {
		l := strings.TrimSpace(strings.TrimLeft(m[0], "//"))
		ws := strings.Split(l, ",")

		for _, w := range ws {
			labels.Insert(w)
		}
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

func getMentors(path string) ([]Mentor, error) {
	v, err := loadPathContent(path)
	if err != nil {
		return nil, err
	}

	var mentors []Mentor

	if err := json.Unmarshal(v, &mentors); err != nil {
		return nil, err
	}

	return mentors, nil
}

func getLabelAssignee(mentorPath string, labels sets.String) (string, error) {
	mentors, err := getMentors(mentorPath)
	if err != nil {
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

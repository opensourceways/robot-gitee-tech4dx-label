package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	wordsRegStr = "合规提升"
)

var (
	wordsReg = regexp.MustCompile(wordsRegStr)
)

type Mentor struct {
	Words []string `json:"words"`
	Label string   `json:"label"`
	Name  string   `json:"name"`
}

func (bot *robot) handleIssueCreate(e *sdk.IssueEvent, cfg *botConfig, log *logrus.Entry) error {
	if e.GetAction() != sdk.ActionOpen {
		return nil
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

	org, repo := e.GetOrgRepo()
	number := e.GetIssueNumber()

	labels := e.GetIssueLabelSet()

	assignee := e.GetIssue().GetAssignee().GetLogin()

	return bot.handleIssueTitleWords(assignee, labels, e.GetIssue().GetTitle(), org, repo, number, cfg)
}

func (bot *robot) handleIssueNoLabels(e *sdk.IssueEvent, cfg *botConfig, log *logrus.Entry) error {
	issue := e.GetIssue()
	org, repo := e.GetOrgRepo()
	number := e.GetIssueNumber()

	labels := sets.NewString()

	assignee := issue.GetAssignee().GetLogin()

	return bot.handleIssueTitleWords(assignee, labels, e.GetIssue().GetTitle(), org, repo, number, cfg)
}

func (bot *robot) handleIssueHasLabels(e *sdk.IssueEvent, cfg *botConfig, log *logrus.Entry) error {
	org, repo := e.GetOrgRepo()
	number := e.GetIssueNumber()

	labels := e.GetIssueLabelSet()
	assignee, err := getLabelAssignee(cfg.MenterPath, labels)
	if err != nil {
		return err
	}

	return bot.handleIssueTitleWords(assignee, labels, e.GetIssue().GetTitle(), org, repo, number, cfg)
}

func (bot *robot) handleIssueTitleWords(
	assignee string,
	labesToAdd sets.String,
	title string,
	org string,
	repo string,
	number string,
	cfg *botConfig,
) error {
	words := parseWordsFromIssue(title)
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

	return bot.addMutilLabels(org, repo, number, labelsFind)
}

func (bot *robot) addMutilLabels(org, repo, number string, labels sets.String) error {
	repoLabels, err := bot.getRepoLabelSet(org, repo)
	if err != nil {
		return err
	}

	canAdd := labels.Intersection(repoLabels)

	return bot.cli.AddMultiIssueLabel(org, repo, number, canAdd.UnsortedList())
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

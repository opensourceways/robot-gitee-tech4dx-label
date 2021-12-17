package main

import (
	"fmt"

	"github.com/opensourceways/community-robot-lib/config"
)

type configuration struct {
	ConfigItems []botConfig `json:"config_items,omitempty"`
}

func (c *configuration) configFor(org, repo string) *botConfig {
	if c == nil {
		return nil
	}

	items := c.ConfigItems
	v := make([]config.IRepoFilter, len(items))

	for i := range items {
		v[i] = &items[i]
	}

	if i := config.Find(org, repo, v); i >= 0 {
		return &items[i]
	}

	return nil
}

func (c *configuration) Validate() error {
	if c == nil {
		return nil
	}

	items := c.ConfigItems
	for i := range items {
		if err := items[i].validate(); err != nil {
			return err
		}
	}

	return nil
}

func (c *configuration) SetDefault() {
	if c == nil {
		return
	}

	Items := c.ConfigItems
	for i := range Items {
		Items[i].setDefault()
	}
}

type botConfig struct {
	config.RepoFilter

	// MenterPath the file  path of menter.json
	MenterPath string `json:"menter_file_path" required:"true"`

	// IssueCommetTplPath the file path of issue comment template
	IssueCommetTplPath string `json:"issue_comment_tpl_path" required:"true"`

	// DescisionTplPath the file path of descision template
	DescisionTplPath string `json:"descision_tpl_path" required:"true"`

	// ParitiTplPath  the file path of pariti template
	ParitiTplPath string `json:"pariti_tpl_path" required:"true"`

	// ParitiAITplPath the file path of pariti AI template
	ParitiAITplPath string `json:"pariti_ai_tpl_path" required:"true"`

	// AssignTplPath the file path of assign template
	AssignTplPath string `json:"assign_tpl_path" required:"true"`
}

func (c *botConfig) setDefault() {
}

func (c *botConfig) validate() error {
	cfgErr := "%s configuration items must be configured"

	if c.MenterPath == "" {
		return fmt.Errorf(cfgErr, "menter_file_path ")
	}

	if c.IssueCommetTplPath == "" {
		return fmt.Errorf(cfgErr, "issue_comment_tpl_path")
	}

	if c.DescisionTplPath == "" {
		return fmt.Errorf(cfgErr, "descision_tpl_path")
	}

	if c.ParitiTplPath == "" {
		return fmt.Errorf(cfgErr, "pariti_tpl_path")
	}

	if c.ParitiAITplPath == "" {
		return fmt.Errorf(cfgErr, "pariti_ai_tpl_path")
	}

	if c.AssignTplPath == "" {
		return fmt.Errorf(cfgErr, "assign_tpl_path")
	}

	return c.RepoFilter.Validate()
}

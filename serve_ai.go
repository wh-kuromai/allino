package allino

import (
	"strings"
)

type AIConfig struct {
	DefaultModel    string `json:"default_model"`
	ToolMaxBodySize int    `json:"tool_max_body_size"`
	ToolMaxLoop     int    `json:"tool_max_loop"`

	ChatGPT ChatGPTConfig `json:"chatgpt"`
}

type ChatGPTConfig struct {
	APIKey         string `json:"apikey"`
	ResponseAPIURL string `json:"response_api_url"`
}

func (c *AIConfig) Select(model ...string) AI {
	var selectedModel string
	if len(model) > 0 {
		selectedModel = model[0]
	}

	if selectedModel == "" {
		selectedModel = c.DefaultModel
	}

	if selectedModel == "" {
		return nil
	}

	sidx := strings.Index(selectedModel, "/")
	if sidx <= 0 {
		return nil
	}

	provider := selectedModel[:sidx]
	pmodel := selectedModel[sidx+1:]

	return NewAI(c, provider, pmodel)
}

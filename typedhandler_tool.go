package allino

import "github.com/wh-kuromai/jsonino"

type ChatGPTNamespace struct {
	Type        string             `json:"type,omitempty"`
	Name        string             `json:"name,omitempty"`
	Description string             `json:"description,omitempty"`
	Tools       []*ChatGPTFunction `json:"tools,omitempty"`
}

type ChatGPTFunction struct {
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Strict      bool            `json:"strict,omitempty"`
	Parameters  *jsonino.Schema `json:"parameters,omitempty"`
}

type ChatGPTToolChoice struct {
	Type  string             `json:"type,omitempty"`
	Mode  string             `json:"mode,omitempty"`
	Tools []*ChatGPTFunction `json:"description,omitempty"`
}

type ChatGPTCall struct {
	Id        string `json:"id,omitempty"`
	CallId    string `json:"call_id,omitempty"`
	Type      string `json:"type,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"` // encoded json
}

type ChatGPTCallResult struct {
	Type   string `json:"type,omitempty"`
	CallId string `json:"call_id,omitempty"`
	Output string `json:"output,omitempty"`
}

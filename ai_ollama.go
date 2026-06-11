package allino

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Ollama struct {
	config *AIConfig
	model  string
	url    string
}

func init() {
	RegisterAI("ollama", func(config *AIConfig, model string) AI {
		return &Ollama{
			config: config,
			model:  model,
			url:    "http://localhost:11434/api/chat",
		}
	})
}

type ollamaRequest struct {
	Model    string           `json:"model"`
	Messages []map[string]any `json:"messages"`
	Tools    []ollamaTool     `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type ollamaResponse struct {
	Message struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls"`
	} `json:"message"`
}

type ollamaToolCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

func buildTools(tools []TypedHandler) ([]ollamaTool, error) {
	var result []ollamaTool

	for _, tool := range tools {
		schema, err := tool.InputSchema()
		if err != nil {
			return nil, err
		}

		buf, err := json.Marshal(schema)
		if err != nil {
			return nil, err
		}

		var params map[string]any

		if err := json.Unmarshal(buf, &params); err != nil {
			return nil, err
		}

		result = append(result, ollamaTool{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        tool.Options().Name,
				Description: tool.Options().Description,
				Parameters:  params,
			},
		})
	}

	return result, nil
}

func (o *Ollama) Inference(
	r *Request,
	messages []map[string]any,
	caller TypedHandler,
	tools []TypedHandler,
) (string, error) {

	toolDefs, err := buildTools(tools)
	if err != nil {
		return "", err
	}

	for i := 0; i < 10; i++ {

		reqBody := ollamaRequest{
			Model:    o.model,
			Messages: messages,
			Tools:    toolDefs,
			Stream:   false,
		}

		buf, err := json.Marshal(reqBody)
		if err != nil {
			return "", err
		}

		req, err := http.NewRequest(
			http.MethodPost,
			o.url,
			bytes.NewReader(buf),
		)
		if err != nil {
			return "", err
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := r.HttpClient().Do(req)
		if err != nil {
			return "", err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return "", err
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf(
				"ollama: %s",
				string(body),
			)
		}

		var result ollamaResponse

		if err := json.Unmarshal(body, &result); err != nil {
			return "", err
		}

		assistantMsg := map[string]any{
			"role":    "assistant",
			"content": result.Message.Content,
		}

		if len(result.Message.ToolCalls) == 0 {
			return result.Message.Content, nil
		}

		var toolCalls []any

		for _, tc := range result.Message.ToolCalls {

			toolCalls = append(toolCalls, map[string]any{
				"function": map[string]any{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			})
		}

		assistantMsg["tool_calls"] = toolCalls

		messages = append(messages, assistantMsg)

		for _, tc := range result.Message.ToolCalls {

			tool := findTool(
				tools,
				tc.Function.Name,
			)

			if tool == nil {
				continue
			}

			inputBuf, err := json.Marshal(
				tc.Function.Arguments,
			)
			if err != nil {
				return "", err
			}

			input, err := tool.UnmarshalInput(inputBuf)
			if err != nil {
				return "", err
			}

			output, err := tool.Handlefunc(
				r,
				input,
			)
			if err != nil {
				return "", err
			}

			outputBuf, err := json.Marshal(output)
			if err != nil {
				return "", err
			}

			messages = append(messages,
				map[string]any{
					"role":    "tool",
					"name":    tc.Function.Name,
					"content": string(outputBuf),
				},
			)
		}
	}

	return "", fmt.Errorf("tool call loop exceeded")
}

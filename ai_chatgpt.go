package allino

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

type ChatGPT struct {
	apiKey string
	model  string
	config *AIConfig
}

func init() {
	_ = RegisterAI("chatgpt", func(config *AIConfig, model string) AI {
		return &ChatGPT{
			apiKey: config.ChatGPT.APIKey,
			model:  model,
			config: config,
		}
	})
}

func (c *ChatGPT) Inference(
	r *Request,
	messages []map[string]any,
	caller TypedHandler,
	tools []TypedHandler,
) (string, error) {
	logger := r.Logger()
	logger.Debug("chatgpt inference start",
		zap.Int("message_count", len(messages)),
	)

	depth := 0
	var responseID string

	for {
		logger.Debug("chatgpt responses create",
			zap.Int("depth", depth),
			zap.String("previous_response_id", responseID),
		)
		resp, err := c.responsesCreate(r, responseID, messages, tools)
		if err != nil {
			return "", err
		}

		var nextInput []map[string]any

		for _, item := range resp.Output {

			switch item.Type {

			case "message":
				for _, content := range item.Content {
					if content.Type == "output_text" {
						logger.Info("chatgpt assistant response",
							zap.String("text", content.Text),
						)
						return content.Text, nil
					}
				}

			case "function_call":
				logger.Info("chatgpt tool call",
					zap.String("tool", item.Name),
					zap.String("call_id", item.CallID),
					zap.String("arguments", item.Arguments),
				)

				handler := findTool(tools, item.Name)
				if handler == nil {
					return "", fmt.Errorf("tool not found: %s", item.Name)
				}

				inputObj, err := handler.UnmarshalInput(
					[]byte(item.Arguments),
				)
				if err != nil {
					return "", err
				}

				output, err := handler.Handlefunc(r, inputObj)
				if err != nil {
					logger.Warn("chatgpt tool error",
						zap.String("tool", item.Name),
						zap.Error(err),
					)
				} else {
					logger.Info("chatgpt tool success",
						zap.String("tool", item.Name),
					)
				}

				var outputJSON []byte

				if err != nil {
					outputJSON, _ = json.Marshal(
						map[string]any{
							"error": err.Error(),
						},
					)

				} else {
					outputJSON, err = json.Marshal(output)
					if err != nil {
						return "", err
					}
				}

				if c.config.ToolMaxBodySize > 0 && len(outputJSON) > c.config.ToolMaxBodySize {
					outputJSON = outputJSON[:c.config.ToolMaxBodySize]
				}

				nextInput = append(nextInput,
					map[string]any{
						"type":    "function_call_output",
						"call_id": item.CallID,
						"output":  string(outputJSON),
					},
				)
			}
		}

		if len(nextInput) == 0 {
			return "", fmt.Errorf("no output returned")
		}

		messages = nextInput
		depth++

		if c.config.ToolMaxLoop > 0 && depth >= c.config.ToolMaxLoop {
			return "", fmt.Errorf("tool max loop exceeded")
		}
	}
}

func (c *ChatGPT) responsesCreate(
	r *Request,
	previousResponseID string,
	input any,
	tools []TypedHandler,
) (*ChatGPTResponsesResponse, error) {

	var toolDefs []map[string]any

	for _, tool := range tools {

		schema, err := tool.InputSchema()
		if err != nil {
			return nil, err
		}

		js, err := json.Marshal(schema)
		if err != nil {
			return nil, err
		}

		var params any
		if err := json.Unmarshal(js, &params); err != nil {
			return nil, err
		}

		toolDefs = append(toolDefs, map[string]any{
			"type":        "function",
			"name":        tool.Options().Name,
			"description": tool.Options().Description,
			"parameters":  params,
		})
	}

	body := map[string]any{
		"model": c.model,
		"input": input,
		"text": map[string]any{
			"format": map[string]any{
				"type": "json_object",
			},
		},
	}

	if len(toolDefs) > 0 {
		body["tools"] = toolDefs
	}

	if previousResponseID != "" {
		body["previous_response_id"] = previousResponseID
	}

	buf, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		c.config.ChatGPT.ResponseAPIURL,
		bytes.NewReader(buf),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.HttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf(
			"openai error: %s",
			string(raw),
		)
	}

	var result ChatGPTResponsesResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

type ChatGPTResponsesResponse struct {
	ID     string              `json:"id"`
	Output []ChatGPTOutputItem `json:"output"`
}

type ChatGPTOutputItem struct {
	Type string `json:"type"`

	// function_call
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	CallID    string `json:"call_id,omitempty"`

	// message
	Content []ChatGPTOutputContent `json:"content,omitempty"`
}

type ChatGPTOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

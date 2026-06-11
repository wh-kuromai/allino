package allino

import (
	"errors"
	"strings"

	"github.com/goccy/go-yaml"
)

type AI interface {
	Inference(
		r *Request,
		messages []map[string]any,
		caller TypedHandler,
		tools []TypedHandler,
	) (string, error)
}

var registeredAI = map[string]func(config *AIConfig, model string) AI{}

func RegisterAI(provider string, fn func(config *AIConfig, model string) AI) error {
	registeredAI[provider] = fn
	return nil
}

func NewAI(config *AIConfig, provider, model string) AI {
	fn, ok := registeredAI[provider]
	if ok {
		return fn(config, model)
	}
	return nil
}

func NewTypedAI[T, U any](option HandlerOption, model, prompt string, tools []TypedHandler) *GenericTypedHandler[T, U, error] {
	fm, promptbody, ok := SplitFrontMatter(prompt)
	if ok {
		var sfm SkillFrontMatter
		err := yaml.Unmarshal([]byte(fm), &sfm)
		if err != nil {
			return nil
		}

		if sfm.Name != "" {
			option.Name = sfm.Name
		}

		if sfm.Description != "" {
			option.Description = sfm.Description
		}

	}

	if promptbody == "" {
		return nil
	}

	var th *GenericTypedHandler[T, U, error]
	th = NewTypedHandler(option,
		func(r *Request, input T) (output U, err error) {
			var zeroU U

			buf, err := yaml.Marshal(input)
			if err != nil {
				return zeroU, err
			}

			promptall := strings.TrimSpace(promptbody) + "\n---\nInput data:\n\n" + string(buf)

			ai := r.AI(model)
			result, err := ai.Inference(r, []map[string]any{
				{
					"role":    "user",
					"content": promptall,
				},
			}, th, tools)
			if err != nil {
				return zeroU, err
			}

			outputa, err := th.UnmarshalOutput([]byte(result))
			if err != nil {
				return zeroU, err
			}

			output, ok = outputa.(U)
			if !ok {
				return zeroU, errors.New("illegal output")
			}
			return output, nil
		},
	)

	return th
}

type SkillFrontMatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func SplitFrontMatter(s string) (fm string, body string, ok bool) {
	lines := strings.Split(s, "\n")

	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", s, false
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"),
				strings.Join(lines[i+1:], "\n"),
				true
		}
	}

	return "", s, false
}

func findTool(
	tools []TypedHandler,
	name string,
) TypedHandler {

	for _, t := range tools {
		if t.Options().Name == name {
			return t
		}
	}
	return nil
}

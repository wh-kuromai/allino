package allino

import (
	"errors"
	"strings"

	"github.com/goccy/go-yaml"
)

type AI interface {
	Inference(
		r *Runtime,
		messages []map[string]any,
		caller Function,
		tools []Function,
	) (string, error)
}

var registeredAI = map[string]func(config *AIConfig, model string) AI{}

func RegisterAIProvider(provider string, fn func(config *AIConfig, model string) AI) error {
	registeredAI[provider] = fn
	return nil
}

func NewAIProvider(config *AIConfig, provider, model string) AI {
	fn, ok := registeredAI[provider]
	if ok {
		return fn(config, model)
	}
	return nil
}

func NewAI[T, U any](option Option, model, prompt string, tools ...Function) *GenericFunction[T, U, error] {
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

	var th *GenericFunction[T, U, error]
	th = NewFunction(option,
		func(r *Runtime, input T) (output U, err error) {
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
	tools []Function,
	name string,
) Function {

	for _, t := range tools {
		if t.Options().Name == name {
			return t
		}
	}
	return nil
}

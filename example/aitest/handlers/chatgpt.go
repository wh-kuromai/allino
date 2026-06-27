package aitest

import (
	"github.com/openai/openai-go"
	"github.com/wh-kuromai/allino"
)

type AITestPromptInput struct {
}

type AITestPromptOutput struct {
}

var TestPrompt = allino.NewAI[AITestPromptInput, AITestPromptOutput](allino.Option{
	Path: "/api/test/chatgpttest",
	Name: "aitest_prompt",
}, "chatgpt/"+openai.ChatModelGPT4_1Mini, `hello`, nil)

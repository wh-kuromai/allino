package handlers

import "github.com/wh-kuromai/allino"

type MCPToolInput struct {
	Message string `json:"message" validate:"required"`
}

type MCPToolOutput struct {
	Echo string `json:"echo"`
}

var MCPToolFunction = allino.NewFunction(
	allino.Option{
		Name:        "mcp_echo",
		Description: "Echoes a message for MCP tool tests.",
		ContentType: allino.JSON,
		MCP:         "tool",
	},
	func(r *allino.Runtime, input *MCPToolInput) (*MCPToolOutput, error) {
		return &MCPToolOutput{Echo: input.Message}, nil
	},
)

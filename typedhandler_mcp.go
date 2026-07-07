package allino

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/gofiber/fiber/v2"
	"github.com/wh-kuromai/jsonino"
	"go.uber.org/zap"
)

const mcpEndpoint = "/mcp"

var mcpRegisteredServers sync.Map

type MCPConfig struct {
	PromptDirs []string `json:"promptDirs"`
}

type mcpMarkdownPrompt struct {
	Name        string
	Description string
	Text        string
	Path        string
}

type mcpMarkdownPromptFrontMatter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

var MCPExtension = NewExtension[MCPConfig, any](
	"mcp",
	&ExtOption{
		OnInit: func(s *Server, virtual *Runtime) error {
			if len(mcpConfig().PromptDirs) == 0 {
				return nil
			}
			registerMCPHandlers(s)
			return nil
		},
		OnFunctionInit: func(s *Server, virtual *Runtime, opt *Option) error {
			if opt == nil || opt.MCP == "" {
				return nil
			}
			registerMCPHandlers(s)
			return nil
		},
	},
)

func registerMCPHandlers(s *Server) {
	if _, loaded := mcpRegisteredServers.LoadOrStore(s, true); loaded {
		return
	}
	s.HandleFiber(http.MethodPost, mcpEndpoint, func(c *fiber.Ctx) error {
		return handleMCPRequest(s, c)
	})
	s.HandleFiber(http.MethodGet, mcpEndpoint, func(c *fiber.Ctx) error {
		return c.JSON(map[string]any{
			"name":      s.Config.AppName,
			"endpoint":  mcpEndpoint,
			"protocol":  "2024-11-05",
			"transport": "streamable-http",
		})
	})
}

func mcpConfig() *MCPConfig {
	for _, ext := range extensionList {
		mcpExt, ok := ext.(*Extension[MCPConfig, any])
		if ok && mcpExt.Option.Name == "mcp" {
			return mcpExt.Config
		}
	}
	return &MCPConfig{}
}

type mcpJSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpJSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpRPCError    `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func handleMCPRequest(s *Server, c *fiber.Ctx) error {
	var req mcpJSONRPCRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(mcpError(nil, -32700, err.Error()))
	}
	hasID := len(req.ID) > 0
	if !hasID {
		req.ID = []byte("null")
	}

	result, err := dispatchMCPRequest(s, NewRuntime(s, c), &req)
	if err != nil {
		return c.JSON(mcpError(req.ID, -32603, err.Error()))
	}

	if !hasID {
		return c.SendStatus(http.StatusAccepted)
	}

	return c.JSON(mcpJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

func dispatchMCPRequest(s *Server, r *Runtime, req *mcpJSONRPCRequest) (any, error) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
				"prompts":   map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    s.Config.AppName,
				"version": s.Config.Version,
			},
		}, nil
	case "notifications/initialized":
		return map[string]any{}, nil
	case "tools/list":
		return mcpToolsList(s)
	case "tools/call":
		return mcpToolCall(s, r, req.Params)
	case "resources/list":
		return mcpResourcesList(s)
	case "resources/read":
		return mcpResourceRead(s, r, req.Params)
	case "prompts/list":
		return mcpPromptsList(s)
	case "prompts/get":
		return mcpPromptGet(s, r, req.Params)
	default:
		return nil, fmt.Errorf("unsupported MCP method: %s", req.Method)
	}
}

func mcpError(id json.RawMessage, code int, msg string) mcpJSONRPCResponse {
	if len(id) == 0 {
		id = []byte("null")
	}
	return mcpJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcpRPCError{
			Code:    code,
			Message: msg,
		},
	}
}

func mcpOptions(s *Server, kind string) []*Option {
	var out []*Option
	for _, h := range s.FunctionCache {
		opt := h.Options()
		if opt.invoker != nil && strings.EqualFold(opt.MCP, kind) && mcpFunctionName(opt) != "" {
			out = append(out, opt)
		}
	}
	for _, h := range s.internalHandlerCache {
		opt := h.Options()
		if opt.invoker != nil && strings.EqualFold(opt.MCP, kind) && mcpFunctionName(opt) != "" {
			out = append(out, opt)
		}
	}
	return out
}

func findMCPOption(s *Server, kind, name string) *Option {
	for _, opt := range mcpOptions(s, kind) {
		if mcpFunctionName(opt) == name {
			return opt
		}
	}
	return nil
}

var mcpNameRe = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func mcpFunctionName(opt *Option) string {
	if opt == nil {
		return ""
	}
	name := opt.Name
	if name == "" {
		name = strings.Trim(opt.Path, "/")
	}
	if name == "" {
		return ""
	}
	name = mcpNameRe.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	return name
}

func mcpInputSchemaMap(opt *Option) (map[string]any, error) {
	if opt.InputType() == nil {
		return nil, fmt.Errorf("MCP function %s has no input type", mcpFunctionName(opt))
	}
	schema, err := jsonino.SchemaFrom(opt.InputType())
	if err != nil {
		return nil, err
	}
	return mcpSchemaToMap(schema)
}

func mcpOutputSchemaMap(opt *Option) (map[string]any, error) {
	if opt.OutputType() == nil {
		return nil, fmt.Errorf("MCP function %s has no output type", mcpFunctionName(opt))
	}
	schema, err := jsonino.SchemaFrom(opt.OutputType())
	if err != nil {
		return nil, err
	}
	return mcpSchemaToMap(schema)
}

func mcpSchemaToMap(schema *jsonino.Schema) (map[string]any, error) {
	buf, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(buf, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{"type": "object"}
	}
	return out, nil
}

func mcpToolsList(s *Server) (any, error) {
	tools := []map[string]any{}
	for _, opt := range mcpOptions(s, "tool") {
		inputSchema, err := mcpInputSchemaMap(opt)
		if err != nil {
			return nil, err
		}
		outputSchema, err := mcpOutputSchemaMap(opt)
		if err != nil {
			return nil, err
		}
		tools = append(tools, map[string]any{
			"name":         mcpFunctionName(opt),
			"description":  opt.Description,
			"inputSchema":  inputSchema,
			"outputSchema": outputSchema,
		})
	}
	return map[string]any{"tools": tools}, nil
}

func mcpToolCall(s *Server, r *Runtime, params json.RawMessage) (any, error) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	opt := findMCPOption(s, "tool", p.Name)
	if opt == nil {
		return nil, fmt.Errorf("MCP tool not found: %s", p.Name)
	}
	output, err := callMCPFunction(s, r, opt, p.Arguments)
	if err != nil {
		return map[string]any{
			"isError": true,
			"content": []map[string]any{{
				"type": "text",
				"text": err.Error(),
			}},
		}, nil
	}
	text, err := marshalMCPText(output)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"structuredContent": output,
		"content": []map[string]any{{
			"type": "text",
			"text": text,
		}},
	}, nil
}

func mcpResourcesList(s *Server) (any, error) {
	resources := []map[string]any{}
	for _, opt := range mcpOptions(s, "resource") {
		name := mcpFunctionName(opt)
		resources = append(resources, map[string]any{
			"uri":         "allino://" + name,
			"name":        name,
			"description": opt.Description,
			"mimeType":    opt.ContentType,
		})
	}
	return map[string]any{"resources": resources}, nil
}

func mcpResourceRead(s *Server, r *Runtime, params json.RawMessage) (any, error) {
	var p struct {
		URI       string          `json:"uri"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	name := strings.TrimPrefix(p.URI, "allino://")
	opt := findMCPOption(s, "resource", name)
	if opt == nil {
		return nil, fmt.Errorf("MCP resource not found: %s", p.URI)
	}
	output, err := callMCPFunction(s, r, opt, p.Arguments)
	if err != nil {
		return nil, err
	}
	text, err := marshalMCPText(output)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"contents": []map[string]any{{
			"uri":      p.URI,
			"mimeType": opt.ContentType,
			"text":     text,
		}},
	}, nil
}

func mcpPromptsList(s *Server) (any, error) {
	prompts := []map[string]any{}
	seen := map[string]bool{}
	for _, opt := range mcpOptions(s, "prompt") {
		schema, err := mcpInputSchemaMap(opt)
		if err != nil {
			return nil, err
		}
		name := mcpFunctionName(opt)
		seen[name] = true
		prompts = append(prompts, map[string]any{
			"name":        name,
			"description": opt.Description,
			"arguments":   mcpPromptArguments(schema),
		})
	}
	markdownPrompts, err := mcpLoadMarkdownPrompts(s)
	if err != nil {
		return nil, err
	}
	for _, prompt := range markdownPrompts {
		if seen[prompt.Name] {
			continue
		}
		seen[prompt.Name] = true
		prompts = append(prompts, map[string]any{
			"name":        prompt.Name,
			"description": prompt.Description,
			"arguments":   []map[string]any{},
		})
	}
	sort.Slice(prompts, func(i, j int) bool {
		return fmt.Sprint(prompts[i]["name"]) < fmt.Sprint(prompts[j]["name"])
	})
	return map[string]any{"prompts": prompts}, nil
}

func mcpPromptArguments(schema map[string]any) []map[string]any {
	props, _ := schema["properties"].(map[string]any)
	requiredList, _ := schema["required"].([]any)
	required := map[string]bool{}
	for _, v := range requiredList {
		if s, ok := v.(string); ok {
			required[s] = true
		}
	}
	args := []map[string]any{}
	for name, raw := range props {
		arg := map[string]any{
			"name":     name,
			"required": required[name],
		}
		if prop, ok := raw.(map[string]any); ok {
			if desc, ok := prop["description"].(string); ok {
				arg["description"] = desc
			}
		}
		args = append(args, arg)
	}
	return args
}

func mcpPromptGet(s *Server, r *Runtime, params json.RawMessage) (any, error) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	opt := findMCPOption(s, "prompt", p.Name)
	if opt != nil {
		output, err := callMCPFunction(s, r, opt, p.Arguments)
		if err != nil {
			return nil, err
		}
		text, err := marshalMCPText(output)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"description": opt.Description,
			"messages": []map[string]any{{
				"role": "user",
				"content": map[string]any{
					"type": "text",
					"text": text,
				},
			}},
		}, nil
	}

	prompt, err := mcpFindMarkdownPrompt(s, p.Name)
	if err != nil {
		return nil, err
	}
	if prompt == nil {
		return nil, fmt.Errorf("MCP prompt not found: %s", p.Name)
	}
	return map[string]any{
		"description": prompt.Description,
		"messages": []map[string]any{{
			"role": "user",
			"content": map[string]any{
				"type": "text",
				"text": prompt.Text,
			},
		}},
	}, nil
}

func callMCPFunction(s *Server, r *Runtime, opt *Option, args json.RawMessage) (any, error) {
	if len(args) == 0 || string(args) == "null" {
		args = []byte("{}")
	}
	r.cache.req_type = REQUEST_HTTP
	r.loggerWith = r.Logger().With(
		zap.String("mcp", opt.MCP),
		zap.String("tool", mcpFunctionName(opt)),
	)
	_, outputJSON, errJSON, syserr := opt.invoker(r, encodeHandlerName(opt), handlerVersion(opt), args, false, func(input any) error {
		if s.Config.System.DisableValidator {
			return nil
		}
		return s.Validator.Struct(input)
	})
	if syserr != nil {
		return nil, syserr
	}
	if len(errJSON) > 0 {
		return nil, fmt.Errorf("%s", errJSON)
	}
	if len(outputJSON) == 0 {
		return nil, nil
	}
	var output any
	if err := json.Unmarshal(outputJSON, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func marshalMCPText(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case []byte:
		return string(t), nil
	default:
		buf, err := json.Marshal(t)
		if err != nil {
			return "", err
		}
		return string(buf), nil
	}
}

func mcpLoadMarkdownPrompts(s *Server) ([]mcpMarkdownPrompt, error) {
	prompts := []mcpMarkdownPrompt{}
	for _, dir := range mcpConfig().PromptDirs {
		root := mcpResolvePath(s, dir)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
				return nil
			}
			prompt, err := mcpLoadMarkdownPrompt(path)
			if err != nil {
				return err
			}
			prompts = append(prompts, prompt)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].Name < prompts[j].Name
	})
	return prompts, nil
}

func mcpFindMarkdownPrompt(s *Server, name string) (*mcpMarkdownPrompt, error) {
	prompts, err := mcpLoadMarkdownPrompts(s)
	if err != nil {
		return nil, err
	}
	for _, prompt := range prompts {
		if prompt.Name == name {
			return &prompt, nil
		}
	}
	return nil, nil
}

func mcpResolvePath(s *Server, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	base := s.Config.ConfigDir
	if base == "" {
		base, _ = os.Getwd()
	}
	return filepath.Join(base, path)
}

func mcpLoadMarkdownPrompt(path string) (mcpMarkdownPrompt, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return mcpMarkdownPrompt{}, err
	}
	frontmatter, body, err := mcpParseMarkdownPrompt(buf)
	if err != nil {
		return mcpMarkdownPrompt{}, fmt.Errorf("%s: %w", path, err)
	}
	name := frontmatter.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	name = mcpNameRe.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	return mcpMarkdownPrompt{
		Name:        name,
		Description: frontmatter.Description,
		Text:        strings.TrimSpace(body),
		Path:        path,
	}, nil
}

func mcpParseMarkdownPrompt(buf []byte) (mcpMarkdownPromptFrontMatter, string, error) {
	text := strings.ReplaceAll(string(buf), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return mcpMarkdownPromptFrontMatter{}, text, nil
	}
	rest := strings.TrimPrefix(text, "---\n")
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return mcpMarkdownPromptFrontMatter{}, "", fmt.Errorf("unterminated YAML frontmatter")
	}
	yamlText := rest[:idx]
	body := rest[idx+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")

	var frontmatter mcpMarkdownPromptFrontMatter
	if strings.TrimSpace(yamlText) != "" {
		if err := yaml.Unmarshal([]byte(yamlText), &frontmatter); err != nil {
			return mcpMarkdownPromptFrontMatter{}, "", err
		}
	}
	return frontmatter, body, nil
}

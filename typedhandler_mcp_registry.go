package allino

import "sort"

type MCPRegistry interface {
	Summary() MCPSummary
	Tools() ([]MCPToolInfo, error)
	Resources() ([]MCPResourceInfo, error)
	Prompts() ([]MCPPromptInfo, error)
}

type MCPSummary struct {
	Registered            bool   `json:"registered"`
	Endpoint              string `json:"endpoint"`
	Protocol              string `json:"protocol"`
	Transport             string `json:"transport"`
	ResourceScheme        string `json:"resourceScheme"`
	ResourceHost          string `json:"resourceHost"`
	PromptDirCount        int    `json:"promptDirCount"`
	ResourceDirCount      int    `json:"resourceDirCount"`
	FunctionToolCount     int    `json:"functionToolCount"`
	FunctionResourceCount int    `json:"functionResourceCount"`
	FunctionPromptCount   int    `json:"functionPromptCount"`
}

type MCPToolInfo struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	InputSchema  any    `json:"inputSchema,omitempty"`
	OutputSchema any    `json:"outputSchema,omitempty"`
}

type MCPResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

type MCPPromptInfo struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Arguments   []MCPPromptArgInfo `json:"arguments,omitempty"`
}

type MCPPromptArgInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

type mcpRegistryView struct {
	server *Server
}

func (s *Server) MCPRegistry() MCPRegistry {
	if s == nil {
		return nil
	}
	return &mcpRegistryView{server: s}
}

func (r *mcpRegistryView) Summary() MCPSummary {
	config := mcpConfig()
	_, registered := mcpRegisteredServers.Load(r.server)
	return MCPSummary{
		Registered:            registered,
		Endpoint:              mcpEndpoint(),
		Protocol:              "2024-11-05",
		Transport:             "streamable-http",
		ResourceScheme:        mcpResourceScheme(),
		ResourceHost:          mcpResourceHost(),
		PromptDirCount:        len(config.PromptDirs),
		ResourceDirCount:      len(config.ResourceDirs),
		FunctionToolCount:     len(mcpOptions(r.server, "tool")),
		FunctionResourceCount: len(mcpOptions(r.server, "resource")),
		FunctionPromptCount:   len(mcpOptions(r.server, "prompt")),
	}
}

func (r *mcpRegistryView) Tools() ([]MCPToolInfo, error) {
	out := []MCPToolInfo{}
	for _, opt := range mcpOptions(r.server, "tool") {
		inputSchema, err := mcpInputSchemaMap(opt)
		if err != nil {
			return nil, err
		}
		outputSchema, err := mcpOutputSchemaMap(opt)
		if err != nil {
			return nil, err
		}
		out = append(out, MCPToolInfo{
			Name:         mcpFunctionName(opt),
			Description:  opt.Description,
			InputSchema:  inputSchema,
			OutputSchema: outputSchema,
		})
	}
	return out, nil
}

func (r *mcpRegistryView) Resources() ([]MCPResourceInfo, error) {
	out := []MCPResourceInfo{}
	seen := map[string]bool{}
	for _, opt := range mcpOptions(r.server, "resource") {
		name := mcpFunctionName(opt)
		uri := "allino://" + name
		seen[uri] = true
		out = append(out, MCPResourceInfo{
			URI:         uri,
			Name:        name,
			Description: opt.Description,
			MIMEType:    opt.ContentType,
		})
	}
	localResources, err := mcpLoadLocalResources(r.server)
	if err != nil {
		return nil, err
	}
	for _, resource := range localResources {
		if seen[resource.URI] {
			continue
		}
		seen[resource.URI] = true
		out = append(out, MCPResourceInfo{
			URI:         resource.URI,
			Name:        resource.Name,
			Description: resource.Description,
			MIMEType:    resource.MIMEType,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].URI < out[j].URI
	})
	return out, nil
}

func (r *mcpRegistryView) Prompts() ([]MCPPromptInfo, error) {
	out := []MCPPromptInfo{}
	seen := map[string]bool{}
	for _, opt := range mcpOptions(r.server, "prompt") {
		schema, err := mcpInputSchemaMap(opt)
		if err != nil {
			return nil, err
		}
		name := mcpFunctionName(opt)
		seen[name] = true
		out = append(out, MCPPromptInfo{
			Name:        name,
			Description: opt.Description,
			Arguments:   mcpPromptArgInfos(schema),
		})
	}
	markdownPrompts, err := mcpLoadMarkdownPrompts(r.server)
	if err != nil {
		return nil, err
	}
	for _, prompt := range markdownPrompts {
		if seen[prompt.Name] {
			continue
		}
		seen[prompt.Name] = true
		out = append(out, MCPPromptInfo{
			Name:        prompt.Name,
			Description: prompt.Description,
			Arguments:   []MCPPromptArgInfo{},
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func mcpPromptArgInfos(schema map[string]any) []MCPPromptArgInfo {
	props, _ := schema["properties"].(map[string]any)
	requiredList, _ := schema["required"].([]any)
	required := map[string]bool{}
	for _, v := range requiredList {
		if s, ok := v.(string); ok {
			required[s] = true
		}
	}
	out := make([]MCPPromptArgInfo, 0, len(props))
	for name, raw := range props {
		arg := MCPPromptArgInfo{
			Name:     name,
			Required: required[name],
		}
		if prop, ok := raw.(map[string]any); ok {
			if desc, ok := prop["description"].(string); ok {
				arg.Description = desc
			}
		}
		out = append(out, arg)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

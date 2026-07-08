package allino

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

type routeKey struct {
	Method string
	Path   string
}

func printRoute(s *Server) {
	allh := s.RegisteredFunctions()

	fmt.Print(strings.TrimSpace(`
Note:
  - No Response → text/html or redirect
  - Response defined → errors return 400 {"error":{"msg":"string"}}
  - *=required, (default=...) indicates default values
`))
	fmt.Print("\n\n")

	// 👇 1. 重複チェック用マップ
	counts := map[routeKey]int{}

	for _, r := range allh {
		key := routeKey{r.Method, r.Path}
		counts[key]++
	}

	// 👇 2. グルーピング（前のやつ）
	grouped := map[string][]*Option{}
	for _, r := range allh {
		grouped[r.Package] = append(grouped[r.Package], r)
	}

	var packages []string
	for pkg := range grouped {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)

	// 👇 3. 出力
	for _, pkg := range packages {
		fmt.Printf("## %s\n", cleanPkg(pkg))

		handlers := grouped[pkg]

		sort.Slice(handlers, func(i, j int) bool {
			if handlers[i].Path == handlers[j].Path {
				return handlers[i].Method < handlers[j].Method
			}
			return handlers[i].Path < handlers[j].Path
		})

		for _, r := range handlers {
			line, form := generateRouteFromOptions(r)

			key := routeKey{r.Method, r.Path}
			dup := counts[key] > 1

			if r.Summary == "" {
				if dup {
					fmt.Printf("%s  ⚠️ duplicate\n", line)
				} else {
					fmt.Printf("%s\n", line)
				}
			} else {
				if dup {
					fmt.Printf("%s  # %s  ⚠️ duplicate\n", line, r.Summary)
				} else {
					fmt.Printf("%s  # %s\n", line, r.Summary)
				}
			}

			if form != "" {
				fmt.Println(form)
			}
		}

		fmt.Println()
	}
}
func printOpenAPI(s *Server) {

	schema := s.GenerateOpenAPI()

	//jsonBytes, _ := json.MarshalIndent(schema, "", "  ")
	//var intermediate OpenAPI
	//json.Unmarshal(jsonBytes, &intermediate)
	//fmt.Print(string(jsonBytes))
	yamlBytes, _ := yaml.Marshal(schema)
	fmt.Print(string(yamlBytes))
}

func printMCP(s *Server) {
	config := mcpConfig()
	endpoint := mcpEndpoint()
	enabled := len(mcpOptions(s, "tool")) > 0 ||
		len(mcpOptions(s, "resource")) > 0 ||
		len(mcpOptions(s, "prompt")) > 0 ||
		len(config.PromptDirs) > 0 ||
		len(config.ResourceDirs) > 0

	fmt.Print("MCP Endpoint:\n")
	fmt.Printf("  POST %s\n", endpoint)
	fmt.Print("Metadata Endpoint:\n")
	fmt.Printf("  GET %s\n", endpoint)
	fmt.Print("Protocol:\n")
	fmt.Print("  2024-11-05\n")
	fmt.Print("Transport:\n")
	fmt.Print("  streamable-http\n")
	fmt.Print("Resource URI:\n")
	fmt.Printf("  %s://%s/<relative-path>\n", mcpResourceScheme(), mcpResourceHost())
	fmt.Printf("Enabled:\n  %t\n", enabled)

	if len(config.PromptDirs) > 0 {
		fmt.Print("Prompt Dirs:\n")
		for _, dir := range config.PromptDirs {
			fmt.Printf("  - %s\n", mcpResolvePath(s, dir))
		}
	}
	if len(config.ResourceDirs) > 0 {
		fmt.Print("Resource Dirs:\n")
		for _, dir := range config.ResourceDirs {
			fmt.Printf("  - %s\n", mcpResolvePath(s, dir))
		}
	}

	fmt.Print("\n")
	printMCPTools(s)
	printMCPPrompts(s)
	printMCPResources(s)
}

func printMCPTools(s *Server) {
	result, err := mcpToolsList(s)
	if err != nil {
		fmt.Printf("## Tools\nError: %v\n\n", err)
		return
	}

	tools := mcpListFromResult(result, "tools")
	fmt.Print("## Tools\n")
	if len(tools) == 0 {
		fmt.Print("(none)\n\n")
		return
	}
	for _, item := range tools {
		name := fmt.Sprint(item["name"])
		desc := fmt.Sprint(item["description"])
		if desc != "" {
			fmt.Printf("- %s  # %s\n", name, desc)
		} else {
			fmt.Printf("- %s\n", name)
		}
		if schema, ok := item["inputSchema"].(map[string]any); ok {
			args := mcpSchemaArgs(schema)
			if args != "" {
				fmt.Printf("  Args: %s\n", args)
			}
		}
	}
	fmt.Print("\n")
}

func printMCPPrompts(s *Server) {
	result, err := mcpPromptsList(s)
	if err != nil {
		fmt.Printf("## Prompts\nError: %v\n\n", err)
		return
	}

	prompts := mcpListFromResult(result, "prompts")
	fmt.Print("## Prompts\n")
	if len(prompts) == 0 {
		fmt.Print("(none)\n\n")
		return
	}
	for _, item := range prompts {
		name := fmt.Sprint(item["name"])
		desc := fmt.Sprint(item["description"])
		if desc != "" {
			fmt.Printf("- %s  # %s\n", name, desc)
		} else {
			fmt.Printf("- %s\n", name)
		}
		if args, ok := item["arguments"].([]map[string]any); ok && len(args) > 0 {
			fmt.Printf("  Args: %s\n", mcpPromptArgsLine(args))
		}
	}
	fmt.Print("\n")
}

func printMCPResources(s *Server) {
	result, err := mcpResourcesList(s)
	if err != nil {
		fmt.Printf("## Resources\nError: %v\n\n", err)
		return
	}

	resources := mcpListFromResult(result, "resources")
	fmt.Print("## Resources\n")
	if len(resources) == 0 {
		fmt.Print("(none)\n\n")
		return
	}
	for _, item := range resources {
		uri := fmt.Sprint(item["uri"])
		name := fmt.Sprint(item["name"])
		desc := fmt.Sprint(item["description"])
		if desc != "" {
			fmt.Printf("- %s (%s)  # %s\n", name, uri, desc)
		} else {
			fmt.Printf("- %s (%s)\n", name, uri)
		}
		if mimeType := fmt.Sprint(item["mimeType"]); mimeType != "" {
			fmt.Printf("  MIME: %s\n", mimeType)
		}
	}
	fmt.Print("\n")
}

func mcpListFromResult(result any, key string) []map[string]any {
	root, ok := result.(map[string]any)
	if !ok {
		return nil
	}
	rawList, ok := root[key]
	if !ok {
		return nil
	}
	switch list := rawList.(type) {
	case []map[string]any:
		return list
	case []any:
		out := make([]map[string]any, 0, len(list))
		for _, raw := range list {
			if item, ok := raw.(map[string]any); ok {
				out = append(out, item)
			}
		}
		return out
	default:
		return nil
	}
}

func mcpSchemaArgs(schema map[string]any) string {
	props, _ := schema["properties"].(map[string]any)
	requiredList, _ := schema["required"].([]any)
	required := map[string]bool{}
	for _, v := range requiredList {
		if s, ok := v.(string); ok {
			required[s] = true
		}
	}

	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	args := make([]string, 0, len(names))
	for _, name := range names {
		label := name
		if required[name] {
			label += "*"
		}
		if prop, ok := props[name].(map[string]any); ok {
			if typ := fmt.Sprint(prop["type"]); typ != "" && typ != "<nil>" {
				label += "=" + typ
			}
		}
		args = append(args, label)
	}
	return strings.Join(args, "&")
}

func mcpPromptArgsLine(args []map[string]any) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		name := fmt.Sprint(arg["name"])
		if name == "" {
			continue
		}
		if required, _ := arg["required"].(bool); required {
			name += "*"
		}
		parts = append(parts, name)
	}
	sort.Strings(parts)
	return strings.Join(parts, "&")
}

func printJSON(input []byte) {
	var out bytes.Buffer

	err := json.Indent(&out, input, "", "  ")
	if err != nil {
		return
	}

	fmt.Print(out.String())
}

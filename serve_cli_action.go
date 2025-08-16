package allino

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

func printRoute(s *Server) {
	allh := s.RegisteredTypedHandlers()

	// 1. 最大長を計算
	maxLen := 0
	for _, r := range allh {
		line := fmt.Sprintf("%s %s", r.Method, r.Path)
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	// 2. 出力（パディング）
	for _, r := range allh {
		if r.Summary == "" {
			fmt.Printf("%s %s\n", r.Method, r.Path)
		} else {
			line := fmt.Sprintf("%s %s", r.Method, r.Path)
			fmt.Printf("%-*s   # %s\n", maxLen, line, r.Summary)
		}
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

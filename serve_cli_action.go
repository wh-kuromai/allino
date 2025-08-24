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
		line, _ := generateRouteFromOptions(r)

		//line := fmt.Sprintf("%s %s", r.Method, r.Path)
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	// 2. 出力（パディング）
	for _, r := range allh {
		line, form := generateRouteFromOptions(r)

		if r.Summary == "" {
			fmt.Printf("%s\n", line)
		} else {
			//line := fmt.Sprintf("%s %s", r.Method, r.Path)
			fmt.Printf("%-*s   # %s\n", maxLen, line, r.Summary)
		}
		if form != "" {
			fmt.Println(form)
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

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
	allh := s.RegisteredTypedHandlers()

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
	grouped := map[string][]*HandlerOption{}
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

func printJSON(input []byte) {
	var out bytes.Buffer

	err := json.Indent(&out, input, "", "  ")
	if err != nil {
		return
	}

	fmt.Print(out.String())
}

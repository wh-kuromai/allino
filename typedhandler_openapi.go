package allino

import (
	"fmt"
	"mime/multipart"
	"reflect"
	"strings"

	"github.com/wh-kuromai/jsonino"
)

type OpenAPI struct {
	OpenAPI string                           `json:"openapi" `
	Info    map[string]interface{}           `json:"info"`
	Paths   map[string]map[string]*Operation `json:"paths"`
}
type Operation struct {
	Summary     string               `json:"summary,omitempty"`
	Description string               `json:"description,omitempty"`
	Parameters  []*Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody         `json:"requestBody,omitempty"`
	Responses   map[string]*Response `json:"responses,omitempty"`
}

type Parameter struct {
	Name     string `json:"name"`
	In       string `json:"in"` // "query", "path", etc.
	Required bool   `json:"required"`
	Schema   any    `json:"schema,omitempty"`
}

type RequestBody struct {
	Content map[string]*MediaType `json:"content,omitempty"`
}

type Response struct {
	Description string                `json:"description,omitempty"`
	Content     map[string]*MediaType `json:"content,omitempty"`
	Headers     map[string]*Header    `json:"headers,omitempty"` // 任意
}

type MediaType struct {
	Schema   any                 `json:"schema,omitempty"`
	Example  any                 `json:"example,omitempty"`
	Examples map[string]*Example `json:"examples,omitempty"`
}

type Header struct {
	Description string `json:"description,omitempty"`
	Schema      any    `json:"schema,omitempty"`
}

type Example struct {
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`
	Value       any    `json:"value,omitempty"`
}

func (r *Server) GenerateOpenAPI() *OpenAPI {

	openapi := &OpenAPI{
		OpenAPI: "3.1.0",
		Info: map[string]interface{}{
			"title":   r.Config.AppName,
			"version": r.Config.Version,
		},
		Paths: make(map[string]map[string]*Operation),
	}

	// 1. typedHandlerCache から
	for _, h := range r.typedHandlerCache {
		opt := h.Options()
		addOperationToOpenAPI(opt, openapi)
	}

	// 2. optionsCache から（通常の http.Handler も含める想定）
	for _, opt := range r.optionsCache {
		addOperationToOpenAPI(opt, openapi)
	}
	return openapi
}

func addOperationToOpenAPI(opt *HandlerOption, openapi *OpenAPI) {
	method := strings.ToLower(opt.Method)
	path := opt.Path

	if _, ok := openapi.Paths[path]; !ok {
		openapi.Paths[path] = make(map[string]*Operation)
	}
	openapi.Paths[path][method] = generateOperationFromOptions(opt)
}

func parseParametersAndFormData(t reflect.Type) (
	params []*Parameter,
	formSchema *jsonino.ObjectScheme,
	usesMultipart bool,
) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		var in, name string

		// 優先順位：path > query > form
		switch {
		case field.Tag.Get("path") != "":
			in = "path"
			name = field.Tag.Get("path")
		case field.Tag.Get("query") != "":
			in = "query"
			name = field.Tag.Get("query")
		case field.Tag.Get("form") != "":
			in = "form"
			name = field.Tag.Get("form")
		default:
			continue
		}

		tschema, _ := jsonino.SchemaFrom(field.Type)

		switch in {
		case "path", "query":
			params = append(params, &Parameter{
				Name:     name,
				In:       in,
				Required: true, // path/query はとりあえず required 扱い
				Schema:   tschema,
			})
		case "form":
			if formSchema == nil {
				formSchema = &jsonino.ObjectScheme{
					TypeName:   "object",
					Properties: make(map[string]*jsonino.Schema),
				}
			}

			if field.Type == reflect.TypeOf((*multipart.FileHeader)(nil)) {
				usesMultipart = true
				formSchema.Properties[name] = &jsonino.Schema{
					TypeName: "string",
					Format:   "binary",
				}
			} else {
				formSchema.Properties[name] = tschema
			}
		}
	}
	return
}

func generateOperationFromOptions(opt *HandlerOption) *Operation {
	if opt.Method == "" {
		opt.Method = "GET"
	}
	if opt.ContentType == "" {
		opt.ContentType = "application/json"
	}
	if opt.ResponseStatusCode == 0 {
		opt.ResponseStatusCode = 200
	}

	inputType := opt.inputType
	if inputType.Kind() == reflect.Ptr {
		inputType = inputType.Elem()
	}

	var params []*Parameter
	var formSchema *jsonino.ObjectScheme
	var usesMultipart bool
	if inputType.Kind() == reflect.Struct {
		params, formSchema, usesMultipart = parseParametersAndFormData(inputType)
	}

	var requestBody *RequestBody
	if formSchema != nil {
		if usesMultipart {
			// multipart/form-data に設定
			requestBody = &RequestBody{
				Content: map[string]*MediaType{
					"multipart/form-data": {
						Schema: formSchema,
					},
				},
			}
		} else {
			// application/x-www-form-urlencoded に設定
			requestBody = &RequestBody{
				Content: map[string]*MediaType{
					"application/x-www-form-urlencoded": {
						Schema: formSchema,
					},
				},
			}
		}
	}

	//var err error
	var node *jsonino.Schema
	//var err error
	if opt.outputType != nil {
		node, _ = jsonino.SchemaFrom(opt.outputType)
	}

	mediaType := &MediaType{}
	if node != nil {
		mediaType.Schema = node
	}

	op := &Operation{
		Summary:     opt.Summary,
		Description: opt.Description,
		Parameters:  params,
		RequestBody: requestBody,
		Responses: map[string]*Response{
			fmt.Sprintf("%d", opt.ResponseStatusCode): {
				Description: "Success",
				Content: map[string]*MediaType{
					opt.ContentType: mediaType,
				},
			},
		},
	}

	//var err error
	if opt.errorType != nil {
		pv := opt.errorType
		if pv.Kind() == reflect.Pointer {
			pv = pv.Elem()
		}

		if pv.Kind() == reflect.Struct {
			errnode, err := jsonino.SchemaFrom(opt.errorType)
			if err == nil {
				op.Responses[fmt.Sprintf("%d", opt.ErrorStatusCode)] = &Response{
					Description: "Error",
					Content: map[string]*MediaType{
						opt.ContentType: {
							Schema: errnode,
						},
					},
				}
			}
		}

	}

	return op
}

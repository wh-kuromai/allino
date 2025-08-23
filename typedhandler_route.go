package allino

import (
	"encoding/json"
	"mime/multipart"
	"reflect"

	"github.com/wh-kuromai/jsonino"
)

func generateRouteFromOptions(opt *HandlerOption) (string, string) {
	inputType := opt.inputType
	if inputType.Kind() == reflect.Ptr {
		inputType = inputType.Elem()
	}

	path := opt.Method + " " + opt.Path
	body := ""

	if inputType.Kind() == reflect.Struct {
		params, formSchema, usesMultipart := parseParametersAndFormDataForRoute(inputType)

		if params != "" {
			path += "?" + params
		}
		body = formSchema
		if formSchema != "" && usesMultipart {
			body += " (multipart/form-data)"
		}
	}

	if opt.outputType != nil {
		n, err := jsonino.SchemaFrom(opt.outputType)
		if err == nil {

			sample, _ := json.Marshal(n.SampleJSON())
			if body != "" {
				body += " "
			}
			body += "-> " + string(sample)
		}
	}

	return path, body

}

func parseParametersAndFormDataForRoute(t reflect.Type) (
	params string,
	formSchema string,
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

		//tschema, _ := jsonino.SchemaFrom(field.Type)

		switch in {
		case "query":
			params += name + "=" + field.Type.Name()
		case "form":
			formSchema += name + "=" + field.Type.Name()
			if field.Type == reflect.TypeOf((*multipart.FileHeader)(nil)) {
				usesMultipart = true
			}
		}
	}
	return
}

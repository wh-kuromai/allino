package allino

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
	"reflect"
	"strings"

	"github.com/wh-kuromai/jsonino"
)

func generateRouteFromOptions(opt *Option) (string, string) {
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
		// 👇 ここ追加：[]byte 判定
		t := opt.outputType
		if t == reflect.TypeOf((*[]byte)(nil)).Elem() {
			body += "  Response:\n    Binary (" + opt.ContentType + ")"
			return path, body
		}

		n, err := jsonino.SchemaFrom(opt.outputType)
		if err == nil {

			sample, _ := json.Marshal(n.SampleJSON())
			if body != "" {
				body = "  Request:\n    " + body + "\n"
			}
			body += "  Response:\n    " + string(sample)
		} else {
			//fmt.Println("why error is nil??? ", n, err)
			if body != "" {
				body = "  Request:\n    " + body
			}
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

		// 優先順位：post > path > query > form
		switch {
		case field.Tag.Get("post") != "":
			in = "post"
			name = field.Tag.Get("post")
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
		case "post":
			scm, err := jsonino.SchemaFrom(field.Type)
			if err == nil {
				b, err := json.Marshal(scm.SampleJSON())
				if err == nil {
					formSchema = string(b)
				}
			}
		case "query":
			if i != 0 {
				params += "&"
			}

			params += formatParam(field, name)
		case "form":
			if i != 0 {
				formSchema += "&"
			}

			formSchema += formatParam(field, name)

			if field.Type == reflect.TypeOf((*multipart.FileHeader)(nil)) {
				usesMultipart = true
			}
		}
	}
	return
}

func isRequired(field reflect.StructField) bool {
	v := field.Tag.Get("validate")
	if v == "" {
		return false
	}

	// "required,email" とかにも対応
	for _, rule := range strings.Split(v, ",") {
		if rule == "required" {
			return true
		}
	}
	return false
}

func formatParam(field reflect.StructField, name string) string {
	n := name

	// required
	if isRequired(field) {
		n += "*"
	}

	typ := field.Type.Name()

	// default
	if def := field.Tag.Get("default"); def != "" {
		return fmt.Sprintf("%s=%s(default=%s)", n, typ, def)
	}

	return fmt.Sprintf("%s=%s", n, typ)
}

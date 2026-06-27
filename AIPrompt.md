下記の AI interface を満たす Ollama API を呼び出すコードを実装して。

```go
type AI interface {
	Inference(
		r *Runtime,
		messages []map[string]any,
		caller TypedHandler,
		tools []TypedHandler,
	) (string, error)
}

var registeredAI = map[string]func(config *AIConfig, model string) AI{}

func RegisterAI(provider string, fn func(config *AIConfig, model string) AI) error {
	registeredAI[provider] = fn
	return nil
}

func NewAI(config *AIConfig, provider, model string) AI {
	fn, ok := registeredAI[provider]
	if ok {
		return fn(config, model)
	}
	return nil
}

type AIConfig struct {
	DefaultModel string        `json:"default_model"`
	ChatGPT      ChatGPTConfig `json:"chatgpt"`
}

type ChatGPTConfig struct {
	APIKey string `json:"apikey"`
}

func findTool(
	tools []TypedHandler,
	name string,
) TypedHandler

func (r *Runtime) HttpClient() *http.Client
func (r *Runtime) Logger() *zap.Logger

type TypedHandler interface {
	Options() *HandlerOption
	Copy() TypedHandler
	HandleRequest(r *Runtime)
	Handlefunc(r *Runtime, input any) (output any, err error)

	InputSchema() (*jsonino.Schema, error) // json.Marshal(*jsonino.Schema) will make valid JSONSchema
	OutputSchema() (*jsonino.Schema, error)
	ErrorSchema() (*jsonino.Schema, error)

	UnmarshalInput(buf []byte) (input any, err error)
	UnmarshalOutput(buf []byte) (output any, err error)
	UnmarshalError(buf []byte) (error error, err error)
}
```
package allino_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wh-kuromai/allino"
)

func TestCLI_Help(t *testing.T) {
	app := allino.NewCLI(nil)
	app.Command.SetArgs([]string{})

	output := captureStdout(func() {
		err := app.Command.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "AI-first web framework")
}

func TestCLI_OpenAPI(t *testing.T) {
	app := allino.NewCLI(nil)
	app.Command.SetArgs([]string{"openapi"})

	output := captureStdout(func() {
		app.Run()
	})

	assert.Contains(t, output, "openapi: 3.1.0")
	assert.Contains(t, output, "title: allino")
	assert.Contains(t, output, "/test/authcsrf")
	assert.Contains(t, output, "application/x-www-form-urlencoded")
	assert.Contains(t, output, "summary: Requires authentication and CSRF token")
	assert.Contains(t, output, "echo:")
	assert.Contains(t, output, "user:")
}

func TestCLI_MCP(t *testing.T) {
	app := allino.NewCLI(nil)
	app.Command.SetArgs([]string{"mcp"})

	output := captureStdout(func() {
		app.Run()
	})

	assert.Contains(t, output, "MCP Endpoint:")
	assert.Contains(t, output, "POST /mcp")
	assert.Contains(t, output, "Transport:")
	assert.Contains(t, output, "streamable-http")
	assert.Contains(t, output, "## Tools")
	assert.Contains(t, output, "mcp_echo")
	assert.Contains(t, output, "## Prompts")
	assert.Contains(t, output, "## Resources")
}

func captureStdout(f func()) string {
	// 現在の stdout を退避
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 実行
	f()

	// Writer を閉じて、元に戻す
	_ = w.Close()
	os.Stdout = old

	// 読み取る
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	return buf.String()
}

/*

func TestCLI_ServeAndEcho(t *testing.T) {
	// ポートを固定しないようにランダムにする（環境で競合回避）
	port := "8090" // 例：固定で問題なければこれでもOK
	ready := make(chan struct{})
	// app.Run() を別 goroutine で実行
	done := make(chan struct{})
	go func() {
		defer close(done)

		app := allino.NewCLI(&allino.Config{
			Bind: ":" + port,
			OnListen: func(s *allino.Server) error {
				ready <- struct{}{}
				return nil
			},
		})
		app.Command.SetArgs([]string{"serve"})
		err := app.Command.Execute()
		assert.NoError(t, err)
	}()

	<-ready
	// サーバーが起動するのをちょっと待つ（理想はポーリング）
	time.Sleep(3000 * time.Millisecond)

	// HTTP リクエスト送信
	resp, err := http.Get("http://localhost:" + port + "/test/echo?echo=hello")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "hello")
	// サーバーを止めたい場合は Ctrl+C 相当の仕組みが必要（なければ放置OK）
}
*/

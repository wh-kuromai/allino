package allino_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wh-kuromai/allino"
)

func TestCLI_Help(t *testing.T) {
	app := allino.NewCLI(nil)
	app.SetArgs([]string{})

	output := captureStdout(func() {
		err := app.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "AI-first web framework")
}

func TestCLI_OpenAPI(t *testing.T) {
	app := allino.NewCLI(nil)
	app.SetArgs([]string{"openapi"})

	output := captureStdout(func() {
		err := app.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "openapi: 3.1.0")
	assert.Contains(t, output, "title: allino")
	assert.Contains(t, output, "/test/authcsrf")
	assert.Contains(t, output, "application/x-www-form-urlencoded")
	assert.Contains(t, output, "summary: Requires authentication and CSRF token")
	assert.Contains(t, output, "echo:")
	assert.Contains(t, output, "user:")
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

func TestCLI_ServeAndEcho(t *testing.T) {
	// ポートを固定しないようにランダムにする（環境で競合回避）
	port := "8089" // 例：固定で問題なければこれでもOK

	// app.Run() を別 goroutine で実行
	done := make(chan struct{})
	go func() {
		defer close(done)

		app := allino.NewCLI(&allino.Config{
			Bind: ":" + port,
		})
		app.SetArgs([]string{"serve"})
		err := app.Execute()
		assert.NoError(t, err)
	}()

	// サーバーが起動するのをちょっと待つ（理想はポーリング）
	time.Sleep(1000 * time.Millisecond)

	// HTTP リクエスト送信
	resp, err := http.Get("http://localhost:" + port + "/test/echo?echo=hello")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "hello")
	// サーバーを止めたい場合は Ctrl+C 相当の仕組みが必要（なければ放置OK）
}

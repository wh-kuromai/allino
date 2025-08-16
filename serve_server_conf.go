package allino

import (
	"net/http"
	"time"
)

type ServerConfig struct {
	ReadTimeout       time.Duration `json:"readTimeout"`       // 通常のリクエストの読み取りタイムアウト
	WriteTimeout      time.Duration `json:"writeTimeout"`      // レスポンス書き込みタイムアウト
	IdleTimeout       time.Duration `json:"idleTimeout"`       // Keep-Alive の接続待機時間
	ReadHeaderTimeout time.Duration `json:"readHeaderTimeout"` // ヘッダー読み取りの制限時間（DoS対策）
	MaxHeaderBytes    ByteSize      `json:"maxHeaderBytes"`    // 最大ヘッダーサイズ（デフォルト1MB）
	MaxBodyBytes      ByteSize      `json:"maxBodyBytes"`      // 任意のリミッター中間層で利用する用（任意）
	EnableKeepAlives  bool          `json:"enableKeepAlives"`  // Keep-Alive を有効にするか
}

type HttpsConfig struct {
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

func (c *HttpsConfig) Enabled() bool {
	return c.CertFile != ""
}

func (c *ServerConfig) Setup() (*http.Server, error) {
	sv := &http.Server{
		ReadTimeout:       c.ReadTimeout,
		WriteTimeout:      c.WriteTimeout,
		IdleTimeout:       c.IdleTimeout,
		ReadHeaderTimeout: c.ReadHeaderTimeout,
		MaxHeaderBytes:    int(c.MaxHeaderBytes),
	}

	sv.SetKeepAlivesEnabled(c.EnableKeepAlives)

	return sv, nil
}

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

func echoHandler(w http.ResponseWriter, r *http.Request) {
	body := r.URL.Query().Get("echo")
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(body))
}

func main() {
	socketPath := os.Getenv("PROXYVISOR_PLUGIN_ADDRESS")

	if !strings.HasPrefix(socketPath, "unix:") {
		panic("fail to start plugin: PROXYVISOR_PLUGIN_ADDRESS=" + socketPath)
	}
	socketPath = socketPath[5:]

	fmt.Println(socketPath)

	// 既存のソケットがあれば削除
	if _, err := os.Stat(socketPath); err == nil {
		os.Remove(socketPath)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on unix socket: %v", err)
	}

	// ソケットファイルのパーミッションを設定（必要なら）
	os.Chmod(socketPath, 0666)

	mux := http.NewServeMux()
	mux.HandleFunc("/testplugin/echo", echoHandler)

	server := &http.Server{
		Handler: mux,
	}

	fmt.Println("Listening on Unix socket:", socketPath)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

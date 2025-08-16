package allino

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wh-kuromai/cryptino"
)

func cliEncrypt(envprefix, filepath string) error {
	buf, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	var keybuf []byte
	key := os.Getenv(envprefix + "_SECRET")
	if key == "" {
		keybuf = make([]byte, 32) // 32バイト分のスライスを作成
		_, err := rand.Read(keybuf)
		if err != nil {
			return err
		}

		fmt.Println(envprefix + "_SECRET decryption key generated.")
		fmt.Println("Please store this decryption key securely and safely.")
		fmt.Println("")
		fmt.Println("  "+envprefix+"_SECRET=", base64.RawURLEncoding.EncodeToString(keybuf))
		fmt.Println("")
	} else {
		keybuf, err = base64.RawURLEncoding.DecodeString(key)
		if err != nil {
			return err
		}

		if len(keybuf) != 32 {
			return fmt.Errorf("invalid key "+envprefix+"_SECRET: %s", key)
		}

		fmt.Println(envprefix + "_SECRET env found, use it for encryption.")
		fmt.Println("")
	}

	encbuf, err := cryptino.EncryptByGCM(keybuf, buf)
	if err != nil {
		return err
	}

	filepath = strings.TrimSuffix(filepath, ".yaml")
	filepath = strings.TrimSuffix(filepath, ".json")
	filepath += ".enc"

	fmt.Printf("`%s` successfully generated.\n", filepath)
	fmt.Println("")
	fmt.Printf("  To use this encrypted config file, be sure to set " + envprefix + "_SECRET environment variable")
	fmt.Printf("  and place `secrets.config.enc` in your config directory.")

	return os.WriteFile(filepath, encbuf, 0600)
}

func cliKeygen(s *Server) error {
	config := map[string]any{}
	login := map[string]any{}
	config["login"] = login

	priv, err := cryptino.GenerateKey("ES256")
	if err != nil {
		return err
	}

	login["privatekey"] = priv

	buf, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if s.Config.Redis.URL != "" || s.Config.Redis.ClusterURL != "" {
		login["redis"] = s.Config.Redis
	}

	if s.Config.SQL.DSN != "" {
		login["sql"] = s.Config.SQL
	}

	topath := filepath.Join(s.Config.ConfigDir, "secrets.config.json")

	if fileExists(topath) {
		return errors.New("`secrets.config.json` already exist")
	}

	return os.WriteFile(filepath.Join(s.Config.ConfigDir, "secrets.config.json"), buf, 0600)
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true // ファイルが存在する
	}
	if os.IsNotExist(err) {
		return false // ファイルが存在しない
	}
	// その他のエラー（パーミッションエラーなど）
	fmt.Println("Error checking file:", err)
	return false
}

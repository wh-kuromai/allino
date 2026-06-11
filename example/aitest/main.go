package main

import (
	_ "modernc.org/sqlite"

	"github.com/wh-kuromai/allino"
	_ "github.com/wh-kuromai/allino/example/aitest/handlers"
)

func main() {
	allino.RunCLI(&allino.Config{
		Debug: true,
		//Redis: allino.RedisConfig{
		//	URL: "redis://localhost:6379/0",
		//},
		SQL: allino.SQLConfig{
			Driver: "sqlite",
			//DSN:    "./tmp/test_sqlite" + xid.New().String() + ".db", //"postgresql://testuser@localhost:5432/testdb?sslmode=disable",
		},

		AI: allino.AIConfig{
			ChatGPT: allino.ChatGPTConfig{
				ResponseAPIURL: "http://localhost:8000/api/debug/dump",
			},
		},
	})
}

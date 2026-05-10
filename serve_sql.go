package allino

import (
	"database/sql"
	"os"
	"path/filepath"

	"github.com/rs/xid"
)

type SQLConfig struct {
	Driver       string `json:"driver"`
	DSN          string `json:"dsn"`
	AllowMigrate *bool  `json:"allow_migrate"`
}

func (c *SQLConfig) connect() (*sql.DB, error) {
	if c.Driver != "" {
		if c.Driver == "sqlite" && c.DSN == "" {
			rid := xid.New().String()
			filepath.Join(os.TempDir(), "allino_sqlite_tmp_"+rid+".db")
			trueFlag := true
			c.AllowMigrate = &trueFlag
		}

		db, err := sql.Open(c.Driver, c.DSN)
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	return nil, nil
}

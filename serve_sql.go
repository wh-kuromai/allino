package allino

import (
	"database/sql"
)

type SQLConfig struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

func (c *SQLConfig) connect() (*sql.DB, error) {
	if c.Driver != "" {
		db, err := sql.Open(c.Driver, c.DSN)
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	return nil, nil
}

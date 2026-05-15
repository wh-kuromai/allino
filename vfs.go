package allino

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type VFS interface {
	Get(ctx context.Context, path string) (contentType string, data []byte, err error)
	Set(ctx context.Context, path string, contentType string, data []byte) error
	Delete(ctx context.Context, path string) error
}

type VFSConfig struct {
	Backend string
}

type sqliteVFS struct {
	db *sql.DB
}

func (c *VFSConfig) setup(s *Server) (VFS, error) {
	switch c.Backend {
	case "sqlite":
		if s.SQL == nil {
			return nil, nil
		}

		vfs := &sqliteVFS{
			db: s.SQL,
		}

		allowMigrate := s.Config.SQL.AllowMigrate != nil && *s.Config.SQL.AllowMigrate
		if allowMigrate {
			if err := vfs.migrate(s.appctx); err != nil {
				return nil, err
			}
		}

		return vfs, nil
	}

	return nil, nil
}

func (v *sqliteVFS) migrate(ctx context.Context) error {
	const q = `
CREATE TABLE IF NOT EXISTS vfs (
	path TEXT PRIMARY KEY,
	content_type TEXT NOT NULL,
	data BLOB NOT NULL
);
`
	_, err := v.db.ExecContext(ctx, q)
	return err
}

func (v *sqliteVFS) Get(ctx context.Context, path string) (string, []byte, error) {
	const q = `
SELECT content_type, data
FROM vfs
WHERE path = ?
`

	var contentType string
	var data []byte

	err := v.db.QueryRowContext(ctx, q, path).Scan(&contentType, &data)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, fmt.Errorf("not found: %s", path)
		}
		return "", nil, err
	}

	return contentType, data, nil
}

func (v *sqliteVFS) Set(ctx context.Context, path string, contentType string, data []byte) error {
	const q = `
INSERT INTO vfs (
	path,
	content_type,
	data
)
VALUES (?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
	content_type = excluded.content_type,
	data = excluded.data
`

	_, err := v.db.ExecContext(ctx, q, path, contentType, data)
	return err
}

func (v *sqliteVFS) Delete(ctx context.Context, path string) error {
	const q = `
DELETE FROM vfs
WHERE path = ?
`

	_, err := v.db.ExecContext(ctx, q, path)
	return err
}

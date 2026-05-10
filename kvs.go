package allino

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type GenericKVS[T any] struct {
	backend kvsStrategy
	pool    *ReflectPool[T]
}

func (g *GenericKVS[T]) Get(ctx context.Context, key string) (newt T, err error) {
	var zeroT T
	var buf []byte
	buf, err = g.backend.Get(ctx, key)
	if err != nil {
		return zeroT, err
	}

	return g.pool.New(func(a any) error {
		return json.Unmarshal(buf, a)
	})
}

func (g *GenericKVS[T]) Set(ctx context.Context, key string, value T, ttl ...time.Duration) error {
	vbuf, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return g.backend.Set(ctx, key, vbuf, ttl...)
}

func (g *GenericKVS[T]) Delete(ctx context.Context, key string) error {
	return g.backend.Delete(ctx, key)
}

type kvsStrategy interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl ...time.Duration) error
	Delete(ctx context.Context, key string) error
}

type redisKvsStrategy struct {
	prefix string
	db     redis.UniversalClient
}

func (r *redisKvsStrategy) Get(ctx context.Context, key string) ([]byte, error) {
	return r.db.Get(ctx, r.prefix+key).Bytes()
}
func (r *redisKvsStrategy) Set(ctx context.Context, key string, value []byte, ttl ...time.Duration) error {
	if len(ttl) > 0 {
		return r.db.SetEx(ctx, r.prefix+key, value, ttl[0]).Err()
	}
	return r.db.Set(ctx, r.prefix+key, value, 0).Err()
}
func (r *redisKvsStrategy) Delete(ctx context.Context, key string) error {
	return r.db.Del(ctx, r.prefix+key).Err()
}

func NewRedisKVS[T any](db redis.UniversalClient, prefix string) (*GenericKVS[T], error) {
	return &GenericKVS[T]{
		backend: &redisKvsStrategy{
			prefix: prefix,
			db:     db,
		},
	}, nil
}

func NewSQLKVS[T any](ctx context.Context, db *sql.DB, tablename string, allow_migrate bool) (*GenericKVS[T], error) {
	if allow_migrate {
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
CREATE TABLE %s (
    key TEXT PRIMARY KEY,
    value BLOB NOT NULL,
    expires_at TIMESTAMP NULL
);

CREATE INDEX idx_kvs_expires_at ON kvs(expires_at);
`, tablename))
		if err != nil {
			return nil, err
		}
	}

	return &GenericKVS[T]{
		backend: &sqlKvsStrategy{
			db:        db,
			tablename: tablename,
		},
	}, nil
}

type sqlKvsStrategy struct {
	db        *sql.DB
	tablename string
}

func (s *sqlKvsStrategy) Get(ctx context.Context, key string) ([]byte, error) {
	var value []byte
	var expiresAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT value, expires_at FROM %s WHERE key = ?`, s.tablename),
		key,
	).Scan(&value, &expiresAt)

	if err != nil {
		return nil, err
	}

	// TTLチェック
	if expiresAt.Valid && time.Now().After(expiresAt.Time) {
		// 期限切れなら削除して NotFound 扱い
		_, _ = s.db.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE key = ?
		`, s.tablename), key)
		return nil, sql.ErrNoRows
	}

	return value, nil
}

func (s *sqlKvsStrategy) Set(ctx context.Context, key string, value []byte, ttl ...time.Duration) error {
	var expiresAt *time.Time

	if len(ttl) > 0 {
		t := time.Now().Add(ttl[0])
		expiresAt = &t
	}

	// UPSERT (DBごとに書き方違うので例は MySQL / SQLite 風)
	_, err := s.db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (key, value, expires_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			expires_at = excluded.expires_at
	`, s.tablename), key, value, expiresAt)

	return err
}

func (s *sqlKvsStrategy) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE key = ?`, s.tablename),
		key,
	)
	return err
}

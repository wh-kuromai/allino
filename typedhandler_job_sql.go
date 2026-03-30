package allino

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type jobconsumer = func(r *Request, handler string, injson []byte) (key string, outjson []byte, err []byte, syserr error)

type callSQLStrategy struct {
	name     string
	db       *sql.DB
	issqlite bool
	mu       sync.Mutex

	lastDequeuedId atomic.Int64
	waitInterval   time.Duration
	waitTimeout    time.Duration
	maxretry       int
}

type sqlJobTask struct {
	strategy *callSQLStrategy
	key      string
	handler  string
	meta     *JobMeta
	input    []byte
}

func (t *sqlJobTask) Key() string {
	return t.key
}
func (t *sqlJobTask) Handler() string {
	return t.handler
}
func (t *sqlJobTask) Meta() *JobMeta {
	return t.meta

}
func (t *sqlJobTask) Input() []byte {
	return t.input
}
func (t *sqlJobTask) Success(ctx context.Context, ttl *time.Time, outjson []byte, errjson []byte) (err error) {
	return t.strategy.doneAsync(ctx, t.key, ttl, outjson, errjson)
}
func (t *sqlJobTask) Fail(ctx context.Context) (err error) {
	return t.strategy.Free(ctx, t.key)
}
func (t *sqlJobTask) HeartBeat(ctx context.Context, lease_dur time.Duration) (err error) {
	return t.strategy.LeaseUpdate(ctx, t.key, lease_dur)
}
func (t *sqlJobTask) Requeue(ctx context.Context, delay_sec int) error {
	return t.strategy.requeue(ctx, t.key, delay_sec)
}

func newcallSQLStrategy(sv *Server) *callSQLStrategy {
	s := &callSQLStrategy{
		name:         "sql",
		db:           sv.SQL,
		issqlite:     false,
		waitInterval: sv.Config.JobConfig.WaitInterval,
		waitTimeout:  sv.Config.JobConfig.WaitTimeout,
		maxretry:     sv.Config.JobConfig.MaxRetry,
	}

	s.lastDequeuedId.Store(-1)
	return s
}

func isSQLite(db *sql.DB) bool {
	var v string
	err := db.QueryRow("SELECT sqlite_version()").Scan(&v)
	return err == nil
}

func (c *callSQLStrategy) Init(ctx context.Context) error {
	if isSQLite(c.db) {
		c.issqlite = true
	}

	if c.issqlite {
		c.mu.Lock()
		defer c.mu.Unlock()
	}

	_, err := c.db.Exec(`
	CREATE TABLE IF NOT EXISTS executions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT UNIQUE,
		handler TEXT,
		version TEXT,
		status TEXT NOT NULL,          -- queued / leased / done / error / dead
		parentid TEXT,
		rootid TEXT,
		input BLOB,
		output BLOB,
		error BLOB,
		retry_count INTEGER,
		priority INTEGER NOT NULL,
		ttl DATETIME,
		run_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
    leased_until DATETIME
	);

	CREATE INDEX idx_executions_queued_v2 
	ON executions (handler, priority DESC, created_at ASC)
	WHERE status = 'queued';

	CREATE INDEX idx_exec_lease_timeout
	ON executions(leased_until)
	WHERE status = 'leased';

	CREATE INDEX idx_key
	ON executions(key);
	`)

	return err
}

func (c *callSQLStrategy) Enqueue(
	ctx context.Context,
	handler string,
	meta *JobMeta,
	key string,
	injson []byte,
	delay_sec int,
) (bool, error) {
	if c.issqlite {
		c.mu.Lock()
		defer c.mu.Unlock()
	}

	now := time.Now()

	//res, err := c.db.ExecContext(ctx, `
	//INSERT OR IGNORE INTO executions
	//(key, handler, version, status, parentid, rootid, priority, ttl, created_at, updated_at, run_at, input)
	//VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	//`, key, handler, meta.Version, meta.Status, meta.ParentID, meta.RootID, meta.Priority, meta.TTL, now, now, now, injson)

	res, err := c.db.ExecContext(ctx, `
INSERT INTO executions
(key, handler, version, status, parentid, rootid, priority, ttl, created_at, updated_at, run_at, input)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)

ON CONFLICT(key) DO UPDATE SET
	handler=excluded.handler,
	version=excluded.version,
	status=excluded.status,
	parentid=excluded.parentid,
	rootid=excluded.rootid,
	priority=excluded.priority,
	ttl=excluded.ttl,
	run_at=excluded.run_at,
	input=excluded.input,
	updated_at=excluded.updated_at

WHERE executions.status='done'
  AND executions.ttl < ?
`, key, handler, meta.Version, meta.Status, meta.ParentID, meta.RootID,
		meta.Priority, meta.TTL, now, now, now, injson, now)

	if err != nil {
		return false, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	if rows > 0 {
		return true, nil
	}

	return false, nil
}

func (c *callSQLStrategy) Dequeue(
	ctx context.Context,
	handlers []string,
	leaseDuration time.Duration,
	ema *EMACalculator,
) (jt JobTask, err error) {
	var key string
	var handler string
	var meta JobMeta
	var injson []byte

	if c.issqlite {
		c.mu.Lock()
		defer c.mu.Unlock()
	}

	if len(handlers) == 0 {
		return nil, ErrJobNotFound
	}

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()
	leaseUntil := now.Add(leaseDuration)

	// IN (?, ?, ...) を作る
	placeholders := make([]string, len(handlers))
	for i := range handlers {
		placeholders[i] = "?"
	}

	// args は SQL の ? の順番に合わせる
	// SET leased_until=?, updated_at=?, ... run_at<=?, leased_until<?, handler IN (...)
	args := make([]any, 0, 4+len(handlers))
	args = append(args,
		leaseUntil, // SET leased_until = ?
		now,        // SET updated_at = ?
		now,        // run_at <= ?
		now,        // leased_until < ?
		now,        // ttl < ?
	)
	for _, h := range handlers {
		args = append(args, h) // handler IN (...)
	}

	optimizeWhere := ""

	if !c.issqlite {
		optimizeWhere = "FOR UPDATE SKIP LOCKED"
	}

	// ✅ UPDATE で 1件だけ lease して、その行を RETURNING で回収（これが超重要）
	q := fmt.Sprintf(`
UPDATE executions
SET
  status = 'leased',
  leased_until = ?,
  updated_at = ?
WHERE id = (
  SELECT id
  FROM executions
  WHERE status = 'queued' AND run_at <= ? AND handler IN (%s)
  ORDER BY priority DESC, created_at ASC, id ASC
  LIMIT 1
	%s
)
RETURNING
  id, key, handler, version, status, parentid, rootid, input
`, strings.Join(placeholders, ","), optimizeWhere)

	row := tx.QueryRowContext(ctx, q, args...)
	var id int64
	if err := row.Scan(
		&id,
		&key,
		&handler,
		&meta.Version,
		&meta.Status,
		&meta.ParentID,
		&meta.RootID,
		&injson,
	); err != nil {
		// UPDATE が 0件なら RETURNING も 0行 -> sql.ErrNoRows
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Dequeue
	var cidv int64
	lid := c.lastDequeuedId.Load()
	if lid < 0 {
		cidv = 1
	} else {
		cidv = id - lid
	}

	ema.Update(float64(cidv))
	c.lastDequeuedId.Store(id)

	return &sqlJobTask{
		strategy: c,
		key:      key,
		handler:  handler,
		meta:     &meta,
		input:    injson,
	}, nil
	//return key, handler, meta, injson, nil
}

func (c *callSQLStrategy) Reaping(
	ctx context.Context,
) (err error) {

	now := time.Now()
	_, err = c.db.ExecContext(ctx, `
UPDATE executions
SET 
    status = 'error',
    error = '{"message": "max retries exceeded during reaping"}',
    updated_at = ?
WHERE 
    status = 'leased' 
    AND leased_until < ?
    AND retry_count >= ?;

UPDATE executions
SET 
    status = 'queued',
    leased_until = NULL, -- リースを解除
    updated_at = ?,      -- 現在時刻
    retry_count = COALESCE(retry_count, 0) + 1
WHERE 
    status = 'leased' 
    AND leased_until < ?; -- 期限切れのジョブのみ対象

DELETE FROM executions
WHERE (status = 'done' OR status = 'error')
	AND ttl IS NOT NULL
	AND ttl < ?
	`, now, now, c.maxretry, now, now)

	return err
}

func (c *callSQLStrategy) LeaseUpdate(ctx context.Context, key string, lease_dur time.Duration) (err error) {
	if c.issqlite {
		c.mu.Lock()
		defer c.mu.Unlock()
	}

	now := time.Now()
	leaset := now.Add(lease_dur)
	_, err = c.db.ExecContext(ctx, `
	UPDATE executions
	SET 
  	leased_until = ?,
		updated_at = ?
	WHERE key = ?
	`, leaset, now, key)

	return err
}

func (c *callSQLStrategy) doneAsync(
	ctx context.Context,
	key string,
	ttl *time.Time,
	outjson []byte,
	errjson []byte,
) error {
	if c.issqlite {
		c.mu.Lock()
		defer c.mu.Unlock()
	}

	status := "done"
	if errjson != nil {
		status = "error"
	}

	_, err := c.db.ExecContext(ctx, `
	UPDATE executions
	SET output = ?, error = ?, status = ?, ttl = ?, updated_at = ?
	WHERE key = ?
	`, outjson, errjson, status, ttl, time.Now(), key)

	//a, b := res.RowsAffected()
	//fmt.Println("doneasync", a, b)
	//
	//c.List(ctx, []string{"done"}, 20, 0)

	return err
}

func (c *callSQLStrategy) Free(
	ctx context.Context,
	key string,
) error {
	if c.issqlite {
		c.mu.Lock()
		defer c.mu.Unlock()
	}

	_, err := c.db.ExecContext(ctx, `
	DELETE FROM executions 
	WHERE key = ?
	`, key)

	return err
}

func (c *callSQLStrategy) Result(
	ctx context.Context,
	key string,
) (JobInfo, []byte, []byte, error) {

	row := c.db.QueryRowContext(ctx, `
	SELECT key, handler, version, status, parentid, rootid, created_at, updated_at, leased_until, output, error
	FROM executions
	WHERE key = ?
	`, key)

	var out, errb []byte
	var status string
	var ji JobInfo
	if err := row.Scan(
		&ji.JobID,
		&ji.Handler,
		&ji.Meta.Version,
		&ji.Meta.Status,
		&ji.Meta.ParentID,
		&ji.Meta.RootID,
		&ji.CreatedAt,
		&ji.UpdatedAt,
		&ji.LeasedUntil,
		&out,
		&errb,
	); err != nil {
		return ji, nil, nil, ErrJobNotFound
	}

	if status != "done" && status != "error" {
		return ji, nil, nil, NewJobPendingError(key, "job not finished yet")
	}

	return ji, out, errb, nil
}

func (c *callSQLStrategy) Wait(
	ctx context.Context,
	key string,
	tw *twWheel,
) (ji JobInfo, output []byte, err []byte, syserr error) {
	var zeroJ JobInfo

	now := time.Now()

	done := make(chan bool, 1)
	tw.Add(c.waitInterval, func() bool {

		ji, output, err, syserr = c.Result(ctx, key)
		if syserr != nil {
			return true
		}

		if ji.Meta.Status == "done" || ji.Meta.Status == "error" {
			done <- true
			return false
		}

		// lease expired
		if ji.Meta.Status == "leased" &&
			ji.LeasedUntil != nil &&
			time.Now().After(*ji.LeasedUntil) {
			ji, output, err, syserr = zeroJ, nil, nil, ErrJobExpired
			done <- true
			return false
		}

		if time.Now().After(now.Add(c.waitTimeout)) {
			ji, output, err, syserr = zeroJ, nil, nil, ErrJobExpired
			done <- true
			return false
		}

		return true
	})

	<-done
	return
}

func (c *callSQLStrategy) Hit(
	ctx context.Context,
	handler string,
	key string,
	injson []byte,
) (JobInfo, []byte, []byte, error) {

	row := c.db.QueryRowContext(ctx, `
	SELECT key, handler, version, status, parentid, rootid, priority, ttl, created_at, updated_at, leased_until, output, error
	FROM executions
	WHERE key = ?
	`, key)

	var ji JobInfo
	var out, errb []byte
	if err := row.Scan(
		&ji.JobID,
		&ji.Handler,
		&ji.Meta.Version,
		&ji.Meta.Status,
		&ji.Meta.ParentID,
		&ji.Meta.RootID,
		&ji.Meta.Priority,
		&ji.Meta.TTL,
		&ji.CreatedAt,
		&ji.UpdatedAt,
		&ji.LeasedUntil,
		&out,
		&errb,
	); err != nil {
		return ji, nil, nil, ErrJobNotFound
	}

	if ji.Meta.Status != "done" && ji.Meta.Status != "error" {
		return ji, nil, nil, NewJobPendingError(key, "job not finished yet")
	}

	now := time.Now()
	if ji.Meta.TTL != nil && now.After(*ji.Meta.TTL) {
		return ji, nil, nil, ErrJobNotFound
	}

	return ji, out, errb, nil
}

func (c *callSQLStrategy) Done(
	ctx context.Context,
	handler string,
	meta *JobMeta,
	key string,
	injson []byte,
	outjson []byte,
	errjson []byte,
) error {
	if c.issqlite {
		c.mu.Lock()
		defer c.mu.Unlock()
	}

	status := "done"
	if errjson != nil {
		status = "error"
	}

	now := time.Now()

	_, err := c.db.ExecContext(ctx, `
	INSERT INTO executions
	(key, handler, version, status, parentid, rootid, priority, ttl, input, output, error, created_at, updated_at, run_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(key) DO UPDATE SET
		output = excluded.output,
		error = excluded.error,
		status = excluded.status,
		updated_at = excluded.updated_at
	`, key, handler, meta.Version, status, meta.ParentID, meta.RootID, meta.Priority, meta.TTL, injson, outjson, errjson, now, now, now)

	return err
}

func (c *callSQLStrategy) List(
	ctx context.Context,
	statuses []string,
	limit, offset int,
) ([]JobInfo, error) {

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	where := ""
	args := make([]any, 0, len(statuses)+2)

	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i, st := range statuses {
			placeholders[i] = "?"
			args = append(args, st)
		}
		where = "WHERE status IN (" + strings.Join(placeholders, ",") + ")"
	}

	args = append(args, limit, offset)

	query := `
	SELECT key, handler, version, status, parentid, rootid, priority, ttl, created_at, updated_at, leased_until, retry_count
	FROM executions
	` + where + `
	ORDER BY created_at DESC
	LIMIT ?
	OFFSET ?
	`

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobinfos := make([]JobInfo, 0, limit)

	for rows.Next() {
		var ji JobInfo
		if err := rows.Scan(
			&ji.JobID,
			&ji.Handler,
			&ji.Meta.Version,
			&ji.Meta.Status,
			&ji.Meta.ParentID,
			&ji.Meta.RootID,
			&ji.Meta.Priority,
			&ji.Meta.TTL,
			&ji.CreatedAt,
			&ji.UpdatedAt,
			&ji.LeasedUntil,
			&ji.RetryCount,
		); err != nil {
			return nil, err
		}
		jobinfos = append(jobinfos, ji)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	//b, _ := json.MarshalIndent(jobinfos, "", "  ")
	//fmt.Println(string(b))

	return jobinfos, nil
}

func (c *callSQLStrategy) requeue(ctx context.Context, key string, delay_sec int) error {
	if c.issqlite {
		c.mu.Lock()
		defer c.mu.Unlock()
	}

	now := time.Now()
	runAt := now.Add(time.Duration(delay_sec) * time.Second)

	_, err := c.db.ExecContext(ctx, `
  UPDATE executions
  SET 
    status = 'queued',
    run_at = ?,
    leased_until = NULL, -- リースを解除
    updated_at = ?,
		retry_count = COALESCE(retry_count, 0) + 1
  WHERE key = ?
  `, runAt, now, key)

	return err
}

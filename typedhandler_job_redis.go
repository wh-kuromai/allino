package allino

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type callRedisStrategy struct {
	server      *Server
	group       string
	consumer    string
	streamTypes map[string]*RedisStreamType
	streams     []string
}

type RedisStreamType struct {
	Stream string
	TTL    time.Duration

	mode     string
	inited   bool
	start    string // "0" : read from start, "$" : read after now
	ids      string // ">" : last, ids : start from ids
	listener func(ctx context.Context, reqid string, msg redis.XMessage) error
	current  string
	lastId   string
	total    int64
}

func (rw *GenericTypedHandler[T, U, E]) call_stream(r *Request, input T, fromcall bool) (output U, err error) {
	var zeroU U

	inJSON, err := json.Marshal(input)
	if err != nil {
		if !r.config.Log.Silent {
			r.logger.Error(
				"job input marshal error",
				zap.String("handler", rw.options.Name),
				zap.Error(err),
			)
		}
		return zeroU, ErrJobInputEncodeFailed
	}

	xaddid, err := r.server.Redis.XAdd(r.Context(), &redis.XAddArgs{
		Stream: r.config.JobConfig.RedisKeyPrefix + rw.options.Name,
		ID:     "*",
		Values: map[string]interface{}{
			"input":   string(inJSON),
			"version": handlerVersion(rw.options),
		},
	}).Result()
	if err != nil {
		if !r.config.Log.Silent {
			r.logger.Error(
				"redis stream xadd error",
				zap.String("handler", rw.options.Name),
				zap.Error(err),
			)
		}
		return zeroU, FatalBackendError
	}

	return zeroU, NewJobPendingError(xaddid, "job fanout accepted")
}

func callRedisInit(s *Server, opt *HandlerOption) error {
	var rst *RedisStreamType
	switch opt.JobMode {
	case JOBMODE_FANOUT:
		rst = &RedisStreamType{
			Stream: s.Config.JobConfig.RedisKeyPrefix + opt.Name,
			start:  "$",
			ids:    ">",
		}

	case JOBMODE_REPLAY:
		fallthrough
	case JOBMODE_REPLAYALL:
		rst = &RedisStreamType{
			Stream: s.Config.JobConfig.RedisKeyPrefix + opt.Name,
			start:  "0",
			ids:    ">",
		}
	}

	if rst != nil {

		if s.callRedisStrategy == nil {
			s.callRedisStrategy = &callRedisStrategy{
				server:      s,
				group:       s.Config.JobConfig.RedisStreamGroupPrefix + s.ServerID(),
				consumer:    s.Config.JobConfig.RedisStreamConsumerPrefix + s.ServerID(),
				streamTypes: make(map[string]*RedisStreamType),
			}
		}

		// OnXGroupCreateMkStream calc init point from backup.
		if opt.Job.OnStreamInit != nil {
			rst.start, _ = opt.Job.OnStreamInit()
		}

		if rst.start == "" {
			rst.start = "$"
		}

		if opt.Job.StreamTTL != 0 {
			rst.TTL = opt.Job.StreamTTL
		}

		rst.mode = opt.JobMode
		rst.listener = func(ctx context.Context, reqid string, msg redis.XMessage) error {
			input := msg.Values["input"]
			version := msg.Values["version"]
			injson, ok := input.(string)
			if !ok {
				return ErrStreamInputDecodeFailed
			}
			r := NewRequest(s, nil)
			r.cache.requestid = reqid
			r.cache.req_type = REQUEST_STREAM

			versionstr, ok := version.(string)
			if !ok {
				return ErrStreamInputDecodeFailed
			}

			updated := false
			if hasMajorOrMinorVersionDiff(versionstr, handlerVersion(opt)) {
				var updatein any
				updated, updatein = opt.Job.OnInputUpgrade(versionstr, time.Now(), []byte(injson))
				if updated {
					updatebuf, err := json.Marshal(updatein)
					if err != nil {
						injson = string(updatebuf)
					}
				}
			}

			opt.consumer(r, encodeHandlerName(opt), versionstr, []byte(injson), false, nil)
			return nil
		}

		s.callRedisStrategy.AddStream(rst)
	}

	return nil
}

func callRedisInitEnd(s *Server) error {
	if s.callRedisStrategy != nil {
		return s.callRedisStrategy.StartListen(s.appctx, s.forcectx)
	}
	return nil
}

func (c *callRedisStrategy) IsTarget(opt *HandlerOption) bool {
	switch opt.JobMode {
	case JOBMODE_FANOUT:
		return true
	case JOBMODE_REPLAY:
		return true
	case JOBMODE_REPLAYALL:
		return true
	}
	return false
}

func (c *callRedisStrategy) Received(ctx context.Context, stream string, msg redis.XMessage, finfn func()) error {
	st := c.streamTypes[stream]
	if st != nil && st.listener != nil {
		c.server.jobManager.Do(func() {
			err := st.listener(ctx, msg.ID, msg)
			if err != nil {
				if !c.server.Config.Log.Silent {
					c.server.Logger.Info(
						"redis stream error",
						zap.Error(err),
					)
				}
			}
			if finfn != nil {
				finfn()
			}
		})
	}
	return nil
}

func (c *callRedisStrategy) AddStream(streamtype *RedisStreamType) error {
	c.streamTypes[streamtype.Stream] = streamtype
	c.streams = makeStreams(c.streamTypes)
	return nil
}

// 修正案
func makeStreams(streamTypes map[string]*RedisStreamType) []string {
	keys := make([]string, 0, len(streamTypes))
	ids := make([]string, 0, len(streamTypes))
	for _, st := range streamTypes {
		keys = append(keys, st.Stream)
		ids = append(ids, st.ids)
	}
	return append(keys, ids...)
}

func (c *callRedisStrategy) StartListen(ctx context.Context, forcedone context.Context) (err error) {
	for _, st := range c.streamTypes {

		// まず現状を確認
		info, _ := c.server.Redis.XInfoStream(ctx, st.Stream).Result()

		if info != nil && st.TTL != 0 {

			// TTL より古いデータを一括削除
			durAgo := time.Now().Add(-st.TTL).UnixMilli()
			thresholdID := fmt.Sprintf("%d-0", durAgo)
			trimmed, err := c.server.Redis.XTrimMinID(ctx, st.Stream, thresholdID).Result()
			if err != nil {
				return fmt.Errorf("failed to destroy group: %w", err)
			}

			if !c.server.Config.Log.Silent && trimmed > 0 {
				c.server.Logger.Info(
					"redis stream xtrim",
					zap.String("stream", st.Stream),
					zap.Int64("trimmed", trimmed),
					zap.Int64("total", st.total),
				)
			}

			info, _ = c.server.Redis.XInfoStream(ctx, st.Stream).Result()
		}

		if info == nil {
			st.lastId = "0"
			st.total = 0
		} else {
			st.inited = true
			st.lastId = info.LastGeneratedID
			st.total = info.Length
		}
	}

	for _, st := range c.streamTypes {
		// 初期化状況のコマンドライン出力
		var processed int64 = 0
		processfn := func() {
			processed++

			// 1000件ごと
			if processed%1000 == 0 || processed == st.total {
				percent := float64(0)
				if st.total > 0 {
					percent = float64(processed) / float64(st.total) * 100
				}

				if !c.server.Config.Log.Silent {
					c.server.Logger.Info(
						"redis stream replay progress",
						zap.String("stream", st.Stream),
						zap.Int64("processed", processed),
						zap.Int64("total", st.total),
						zap.Float64("percent", percent),
					)
				}
			}
		}

		// 既にストリームがある場合には REPLAY を考える
		if st.inited {
			found := false
			infocons, _ := c.server.Redis.XInfoConsumers(ctx, st.Stream, c.group).Result()
			for _, infocon := range infocons {
				if infocon.Name == c.consumer {
					found = true
				}
			}

			switch st.mode {
			case JOBMODE_FANOUT:
				if !found {
					err = c.server.Redis.XGroupCreateMkStream(ctx, st.Stream, c.group, st.start).Err()
					if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
						return fmt.Errorf("failed to create group: %w", err)
					}
				}
			case JOBMODE_REPLAY:
				// JOBMODE_REPLAY の場合には、初回なら REPLAY する。
				if !found {
					err = c.replayUntilLastID(ctx, st.Stream, st.start, st.lastId, processfn)
					if err != nil {
						return fmt.Errorf("failed to replay group: %w", err)
					}

					err = c.server.Redis.XGroupCreateMkStream(ctx, st.Stream, c.group, st.start).Err()
					if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
						return fmt.Errorf("failed to create group: %w", err)
					}
				}
			case JOBMODE_REPLAYALL:
				// JOBMODE_REPLAYALL の場合には、毎回 REPLAY する。
				err = c.replayUntilLastID(ctx, st.Stream, st.start, st.lastId, processfn)
				if err != nil {
					return fmt.Errorf("failed to replay group: %w", err)
				}

				err := c.server.Redis.XGroupDestroy(ctx, st.Stream, c.group).Err()
				err = c.server.Redis.XGroupCreateMkStream(
					ctx,
					st.Stream,
					c.group,
					st.lastId).Err()
				if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
					return fmt.Errorf("failed to create group: %w", err)
				}

			}

		} else {
			// ストリームがない場合には作る
			err = c.server.Redis.XGroupCreateMkStream(ctx, st.Stream, c.group, st.start).Err()
			if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
				return fmt.Errorf("failed to create group: %w", err)
			}
		}

		if !c.server.Config.Log.Silent {
			c.server.Logger.Info(
				"redis stream inited",
				zap.String("stream", st.Stream),
				zap.String("group", c.group),
				zap.Int64("total", st.total),
			)
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:

				if !c.server.Config.Log.Silent {
					c.server.Logger.Debug(
						"redis stream XReadGroup",
						zap.Strings("streams", c.streams),
						zap.String("group", c.group),
						zap.String("consumer", c.consumer),
					)
				}
				// Stream を block listen
				res, err := c.server.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{
					Group:    c.group,
					Consumer: c.consumer,
					Streams:  c.streams,
					Count:    1,
					Block:    5 * time.Second,
					NoAck:    false,
				}).Result()

				if err != nil {
					if err == redis.Nil {
						continue
					}

					if !c.server.Config.Log.Silent {
						c.server.Logger.Error(
							"redis stream XReadGroup error",
							zap.String("group", c.group),
							zap.String("consumer", c.consumer),
							zap.Strings("stream", c.streams),
							zap.Error(err),
						)
					}

					time.Sleep(1 * time.Second)
					continue
				}

				for _, stream := range res {
					for _, msg := range stream.Messages {
						c.Received(ctx, stream.Stream, msg, func() {
							// ACK
							err := c.server.Redis.XAck(
								forcedone,
								stream.Stream,
								c.group,
								msg.ID,
							).Err()

							if err != nil {
								if !c.server.Config.Log.Silent {
									c.server.Logger.Error(
										"redis stream XAck error",
										zap.String("stream", stream.Stream),
										zap.String("group", c.group),
										zap.String("consumer", c.consumer),
										zap.Error(err),
									)
								}
							}
						})
					}
				}
			}
		}
	}()
	return nil
}

func (c *callRedisStrategy) replayUntilLastID(
	ctx context.Context,
	stream string,
	startId string,
	lastId string,
	fn func(),
) error {
	// 空stream
	if lastId == "0" || lastId == "" {
		return nil
	}
	if startId == "$" || startId == "" {
		return nil
	}

	current := startId

	for {
		if !c.server.Config.Log.Silent {
			c.server.Logger.Debug(
				"redis stream XRead start",
				zap.String("stream", stream),
				zap.String("current", current),
			)
		}

		res, err := c.server.Redis.XRead(ctx, &redis.XReadArgs{
			Streams: []string{
				stream,
				current,
			},
			Count: 100,
			Block: 1 * time.Second,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				break
			}

			return fmt.Errorf("redis stream XRead error: %w", err)
		}

		if len(res) == 0 {
			break
		}

		done := false

		for _, s := range res {
			for _, msg := range s.Messages {

				// 処理
				c.Received(ctx, s.Stream, msg, func() {})
				fn()

				// 次回開始位置
				current = msg.ID

				// 起動時点の末尾まで到達
				if msg.ID == lastId {
					done = true
					break
				}
			}

			if done {
				break
			}
		}

		if done {
			break
		}
	}
	return nil
}

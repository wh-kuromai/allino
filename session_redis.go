package allino

import (
	"encoding/json"
)

type redisSession struct {
	value    map[string]string
	created  bool
	checkfns map[string]func() (bool, error)
	deferfn  func() error
}

func getRedisSession(r *Request, sid string) (*redisSession, error) {
	if r.cache.sessionredis != nil {
		return r.cache.sessionredis, nil
	}

	result, err := r.Redis().HGetAll(r.Context(), sid).Result()
	if err != nil {
		return nil, err
	}

	sess := &redisSession{
		checkfns: make(map[string]func() (bool, error)),
	}

	if len(result) == 0 {
		sess.created = true
	}

	sess.deferfn = func() error {
		changed := false
		for _, cfn := range sess.checkfns {
			ok, err := cfn()
			if err != nil {
				return err
			}
			if ok {
				changed = true
			}
		}
		if !changed {
			return nil
		}

		setdata := map[string]interface{}{}
		for k, v := range sess.value {
			setdata[k] = v
		}

		err = r.Redis().HSet(r.Context(), sid, setdata).Err()
		return err
	}

	r.Defer(sess.deferfn)

	sess.value = result
	r.cache.sessionredis = sess
	return sess, nil
}

func GetRedisSessionEntry[S any](r *Request, sid, name string) (*S, bool, error) {
	sess, err := getRedisSession(r, sid)
	if err != nil {
		return nil, false, err
	}

	var s S
	entrystr, ok := sess.value[r.cache.sessionname]
	if !ok || len(entrystr) == 0 {
		entrybuf, _ := json.Marshal(&s)
		entrystr = string(entrybuf)
	}

	sess.checkfns[name] = func() (bool, error) {
		endbuf, err := json.Marshal(&s)
		if err != nil {
			return false, err
		}

		if entrystr != string(endbuf) {
			sess.value[name] = string(endbuf)
			return true, nil
		}
		return false, nil
	}

	if !ok || len(entrystr) == 0 {
		return &s, sess.created, nil
	}

	err = json.Unmarshal([]byte(entrystr), &s)
	if err != nil {
		return nil, false, err
	}

	return &s, sess.created, nil
}

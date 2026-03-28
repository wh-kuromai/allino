package allino

import (
	"encoding/json"
	"time"
)

type jobExecutionContext struct {
	r        *Request
	opt      *HandlerOption
	input    any
	fromcall bool

	handler         string
	key             string
	inJSON          []byte
	inJSONMarshaled bool
}

func (c *jobExecutionContext) JobMeta(status string) JobMeta {
	meta := JobMeta{
		Version:  handlerVersion(c.opt),
		Status:   status,
		ParentID: c.r.cache.parentjobid,
		RootID:   c.r.cache.rootjobid,
		Priority: c.opt.Job.Priority,
	}

	if c.opt.Job.CacheExpire != 0 {
		ttl := time.Now().Add(c.opt.Job.CacheExpire)
		meta.TTL = &ttl
	}
	return meta
}

func (c *jobExecutionContext) MarshalCheck() error {
	if c.inJSONMarshaled {
		return nil
	}

	inJSON, err := json.Marshal(c.input)
	c.inJSON = inJSON
	c.inJSONMarshaled = true
	return err
}

func (c *jobExecutionContext) InputJSON() []byte {
	//if c.inJSONMarshaled {
	//	return c.inJSON
	//}
	//
	//inJSON, _ := json.Marshal(c.input)
	//c.inJSON = inJSON
	//c.inJSONMarshaled = true
	//return inJSON

	if !c.inJSONMarshaled {
		panic("jobExecutionContext: InputJSON called before MarshalCheck")
	}
	return c.inJSON
}

func (c *jobExecutionContext) Handler() string {
	if c.handler != "" {
		return c.handler
	}

	c.handler = encodeHandlerName(c.opt)
	return c.handler
}

func (c *jobExecutionContext) JobID() string {
	if c.key != "" {
		return c.key
	}

	marker := ""
	if !c.opt.Job.Dedupe && !c.opt.Job.Cache {
		marker = c.r.RequestID()
	}

	c.key = encodeJobID(c.Handler(), c.input, c.InputJSON(), marker)
	return c.key
}

func (c *jobExecutionContext) EnqueueStatus() string {
	status := "running"
	if c.fromcall && c.opt.Job.Async {
		status = "queued"
	}
	return status
}

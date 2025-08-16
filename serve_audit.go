package allino

import "go.uber.org/zap"

func (r *Request) Audit(msg string, fields ...zap.Field) string {
	uid, displayname, _, err := r.User()
	if err == nil {
		fields = append(fields, zap.String("user_id", uid))
		if displayname != "" {
			fields = append(fields, zap.String("display_name", displayname))
		}
	}

	fields = append(fields, zap.Any("input", r.cache.input))
	r.Logger().Named("audit").Info(msg, fields...)
	return r.RequestID()
}

package allino

import (
	"os"
	"strings"

	"github.com/rs/xid"
	"go.uber.org/zap"
)

func (s *Server) ServerID() string {
	if s.Config.Session.serverid != "" {
		return s.Config.Session.serverid
	}

	s.Config.Session.serveridMu.Lock()
	defer s.Config.Session.serveridMu.Unlock()

	if s.Config.Session.serverid == "" {
		sid := ""
		fname := "allino.serverid"
		if s.Config.Session.ServerIDFile != "" {
			fname = s.Config.Session.ServerIDFile
		}

		_, err := os.Stat(fname)
		if err == nil {
			buf, err2 := os.ReadFile(fname)
			if err2 == nil {
				sid = strings.TrimSpace(string(buf))
			}
		}
		if sid == "" {
			sid = xid.New().String()
			err3 := os.WriteFile(fname, []byte(sid), 0600)

			if err3 != nil && !s.Config.Log.Silent {
				s.Logger.Error("allino.serverid file write error", zap.Error(err))
			}
		}
		s.Config.Session.serverid = sid
	}

	return s.Config.Session.serverid
}

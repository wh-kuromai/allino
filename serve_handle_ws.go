package allino

import (
	"time"

	"github.com/gofiber/fiber/v2"
	websocket "github.com/gofiber/websocket/v2"
)

type WebSocketConfig struct {
	HandshakeTimeout  time.Duration `json:"handshakeTimeout,omitempty"`
	Subprotocols      []string      `json:"subprotocols,omitempty"`
	Origins           []string      `json:"origins,omitempty"`
	ReadBufferSize    ByteSize      `json:"readBufferSize,omitempty"`
	WriteBufferSize   ByteSize      `json:"writeBufferSize,omitempty"`
	EnableCompression *bool         `json:"enableCompression,omitempty"`
}

func (cfg *WebSocketConfig) ToFiberWebSocketConfig() websocket.Config {
	return websocket.Config{
		HandshakeTimeout:  cfg.HandshakeTimeout,
		Subprotocols:      cfg.Subprotocols,
		Origins:           cfg.Origins,
		ReadBufferSize:    int(cfg.ReadBufferSize),
		WriteBufferSize:   int(cfg.WriteBufferSize),
		EnableCompression: *cfg.EnableCompression,
	}
}

type WebsocketRequestHandler func(r *Request) error
type WebsocketConnHandler func(r *Request, conn *websocket.Conn)

func (s *Server) HandleWebsocket(pattern string, requestHandlerFunc WebsocketRequestHandler, connHandlerFunc WebsocketConnHandler, c ...websocket.Config) {
	var cfg websocket.Config
	if len(c) >= 1 {
		cfg = c[0]
	}

	if cfg.HandshakeTimeout == 0 && s.Config.WebSocket.HandshakeTimeout != 0 {
		cfg.HandshakeTimeout = s.Config.WebSocket.HandshakeTimeout
	}
	if len(cfg.Subprotocols) == 0 && len(s.Config.WebSocket.Subprotocols) > 0 {
		cfg.Subprotocols = s.Config.WebSocket.Subprotocols
	}
	if len(cfg.Origins) == 0 && len(s.Config.WebSocket.Origins) > 0 {
		cfg.Origins = s.Config.WebSocket.Origins
	}
	if cfg.ReadBufferSize == 0 && s.Config.WebSocket.ReadBufferSize != 0 {
		cfg.ReadBufferSize = int(s.Config.WebSocket.ReadBufferSize)
	}
	if cfg.WriteBufferSize == 0 && s.Config.WebSocket.WriteBufferSize != 0 {
		cfg.WriteBufferSize = int(s.Config.WebSocket.WriteBufferSize)
	}
	if s.Config.WebSocket.EnableCompression != nil {
		cfg.EnableCompression = *s.Config.WebSocket.EnableCompression
	}

	s.Fiber.Use(pattern, func(c *fiber.Ctx) error {
		req := NewRequest(s, c)
		err := requestHandlerFunc(req)
		if err != nil {
			return nil
		}

		c.Locals("allino", req)
		return c.Next()
	})

	s.Fiber.Use(pattern, websocket.New(func(c *websocket.Conn) {
		defer c.Close()
		req := c.Locals("allino").(*Request)
		connHandlerFunc(req, c)
	}, cfg))
}

package allino

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type spaHandler struct {
	files      fs.FS
	s          *Server
	useStartAt bool
}

func (e *spaHandler) Open(name string) (fs.File, error) {
	if f, err := e.files.Open(name); err == nil {
		return f, nil
	}
	if f, err := e.files.Open(name + ".html"); err == nil {
		return f, nil
	}
	for _, fb := range e.s.Config.Routing.FallbackPaths {
		if f, err := e.files.Open(fb); err == nil {
			return f, nil
		}
	}
	return nil, fs.ErrNotExist
}

func (e *spaHandler) Handler(c *fiber.Ctx) error {
	name := c.Path()

	f, err := e.Open(name)
	if err == nil {
		return e.serveFile(c, f, name)
	}

	if e.s.Config.Routing.ErrorPath != "" {
		ef, err := e.Open(e.s.Config.Routing.ErrorPath)
		if err == nil {
			return e.serveFile(c, ef, name)
		}
	}

	return fiber.ErrNotFound
}

func (e *spaHandler) serveFile(c *fiber.Ctx, file fs.File, name string) error {
	fi, err := file.Stat()
	if err != nil {
		return fiber.ErrInternalServerError
	}

	modTime := e.modTime(file)
	size := fi.Size()
	rs := toReadSeeker(file)

	// Content-Type
	c.Type(getExt(name))
	c.Set("Cache-Control", "public, max-age=3600") // ←任意に調整

	// Last-Modified
	c.Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))

	// ETag（ModTime + Size ベースの簡易実装）
	etag := fmt.Sprintf(`W/"%x-%x"`, modTime.Unix(), size)
	c.Set("ETag", etag)

	// ETag による 304 応答
	if match := c.Get("If-None-Match"); match != "" && match == etag {
		return c.SendStatus(fiber.StatusNotModified)
	}

	// Range ヘッダ処理
	if rangeHeader := c.Get("Range"); rangeHeader != "" {
		start, end, err := parseRange(rangeHeader, size)
		if err != nil {
			return c.Status(fiber.StatusRequestedRangeNotSatisfiable).SendString("Invalid Range")
		}
		c.Status(fiber.StatusPartialContent)
		c.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end-1, size))
		c.Set("Content-Length", fmt.Sprintf("%d", end-start))
		_, err = rs.Seek(start, io.SeekStart)
		if err != nil {
			return fiber.ErrInternalServerError
		}
		return c.SendStream(io.LimitReader(rs, end-start))
	}

	// 通常送信
	c.Set("Content-Length", fmt.Sprintf("%d", size))
	return c.SendStream(rs)
}

func (e *spaHandler) modTime(file fs.File) time.Time {
	if e.useStartAt {
		return e.s.Config.StartAt
	}
	fi, err := file.Stat()
	if err != nil {
		return time.Now()
	}
	return fi.ModTime()
}

func toReadSeeker(file fs.File) io.ReadSeeker {
	if rs, ok := file.(io.ReadSeeker); ok {
		return rs
	}
	buf, _ := io.ReadAll(file)
	return bytes.NewReader(buf)
}

// 拡張子を元に Content-Type を Fiber に教える（省略可、Fiber側が自動判別する場合もある）
func getExt(path string) string {
	ext := filepath.Ext(path)
	if len(ext) > 0 {
		return ext[1:]
	}
	return "html"
}

func NewStaticHandler(s *Server, files fs.FS, stripPath string) fiber.Handler {
	_, embed := files.(embed.FS)

	subfs, _ := fs.Sub(files, stripPath)
	spa := &spaHandler{
		files:      subfs,
		s:          s,
		useStartAt: embed,
	}

	return spa.Handler
}

func parseRange(rng string, size int64) (start, end int64, err error) {
	if !strings.HasPrefix(rng, "bytes=") {
		return 0, 0, fmt.Errorf("invalid range")
	}

	rng = strings.TrimPrefix(rng, "bytes=")
	parts := strings.Split(rng, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format")
	}

	if parts[0] == "" {
		// bytes=-500 → 最後の500バイト
		offset, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		if offset > size {
			offset = size
		}
		return size - offset, size, nil
	}

	start, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}

	if parts[1] == "" {
		// bytes=500- → 500バイト目から最後まで
		end = size
	} else {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		end += 1 // Range は inclusive なので +1 する
	}

	if start >= size || start > end {
		return 0, 0, fmt.Errorf("range out of bounds")
	}

	if end > size {
		end = size
	}

	return start, end, nil
}

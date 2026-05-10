package allino

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileInput struct {
	Path   string    `json:"path" cli:"path"`
	reader io.Reader `json:"-"`
}

type ueResult[U any] struct {
	res map[string]U
	err *MapError
}

func (u *ueResult[U]) Collect(path string, umap map[string]U, err error) {
	if err != nil {
		if u.err == nil {
			u.err = &MapError{Errs: make(map[string]error)}
		}
		var merr *MapError
		if ok := errors.As(err, &merr); ok && merr != nil {
			for k, v := range merr.Errs {
				u.err.Errs[k] = v
			}
		} else {
			u.err.Errs[path] = err
		}
	}

	if u.res == nil {
		u.res = map[string]U{}
	}
	for k, v := range umap {
		u.res[k] = v
	}
}

func (u *ueResult[U]) CollectOne(path string, uone U, err error) {
	if err != nil {
		if u.err == nil {
			u.err = &MapError{Errs: make(map[string]error)}
		}
		u.err.Errs[path] = err
	}

	if u.res == nil {
		u.res = map[string]U{}
	}
	u.res[path] = uone
}

func (u *ueResult[U]) Result() (umap map[string]U, err error) {
	return u.res, u.err
}

type MapError struct {
	Errs map[string]error
}

func (m *MapError) Error() string {
	var s string
	for k, err := range m.Errs {
		s += fmt.Sprintf("%s: %v; ", k, err)
	}
	return s
}

func isTarPath(path string) bool {
	return strings.HasSuffix(path, ".tar")
}

func isGzipPath(path string) bool {
	return strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".gzip")
}

func isTarGzipPath(path string) bool {
	return strings.HasSuffix(path, ".tar.gz") ||
		strings.HasSuffix(path, ".tgz") ||
		strings.HasSuffix(path, ".tar.gzip")
}

func stripGzipSuffix(path string) string {
	switch {
	case strings.HasSuffix(path, ".tar.gz"):
		return strings.TrimSuffix(path, ".gz")
	case strings.HasSuffix(path, ".tar.gzip"):
		return strings.TrimSuffix(path, ".gzip")
	case strings.HasSuffix(path, ".tgz"):
		return strings.TrimSuffix(path, ".tgz") + ".tar"
	case strings.HasSuffix(path, ".gzip"):
		return strings.TrimSuffix(path, ".gzip")
	case strings.HasSuffix(path, ".gz"):
		return strings.TrimSuffix(path, ".gz")
	default:
		return path
	}
}

func NewFileJob[U any](
	opt HandlerOption,
	handler func(r *Request, path string, in io.Reader) (U, error),
) *GenericTypedHandler[*FileInput, map[string]U, error] {
	if opt.Name == "" {
		return nil
	}

	var zeroU U
	var sjob, djob *GenericTypedHandler[*FileInput, map[string]U, error]

	jobhandler := func(r *Request, rin *FileInput) (map[string]U, error) {
		result := &ueResult[U]{}

		// .zip
		// NOTE:
		// archive/zip は ReaderAt が必要なので、ここではパスから開ける場合のみ対応。
		if strings.HasSuffix(rin.Path, ".zip") {
			if rin.reader != nil {
				return nil, fmt.Errorf("zip from io.Reader is not supported: %s", rin.Path)
			}
			zipfile, err := zip.OpenReader(rin.Path)
			if err != nil {
				return nil, err
			}
			defer zipfile.Close()

			for _, zf := range zipfile.File {
				if zf.FileInfo().IsDir() {
					continue
				}

				zffPath := rin.Path + "!" + zf.Name
				zff, err := zf.Open()
				if err != nil {
					result.CollectOne(zffPath, zeroU, err)
					continue
				}

				func() {
					defer zff.Close()
					res, err := sjob.Call(r, &FileInput{
						Path:   zffPath,
						reader: zff,
					})
					result.Collect(zffPath, res, err)
				}()
			}
			return result.Result()
		}

		// source reader を確定
		source := rin.reader
		if source == nil {
			file, err := os.Open(rin.Path)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			source = file
		}

		// .tar.gz / .tgz
		if isTarGzipPath(rin.Path) {
			gzr, err := gzip.NewReader(source)
			if err != nil {
				return nil, err
			}
			defer gzr.Close()

			tr := tar.NewReader(gzr)
			for {
				hdr, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					result.CollectOne(rin.Path, zeroU, err)
					break
				}
				if hdr == nil || hdr.FileInfo().IsDir() {
					continue
				}

				entryPath := rin.Path + "!" + hdr.Name
				res, err := sjob.Call(r, &FileInput{
					Path:   entryPath,
					reader: tr,
				})
				result.Collect(entryPath, res, err)
			}
			return result.Result()
		}

		// .tar
		if isTarPath(rin.Path) {
			tr := tar.NewReader(source)
			for {
				hdr, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					result.CollectOne(rin.Path, zeroU, err)
					break
				}
				if hdr == nil || hdr.FileInfo().IsDir() {
					continue
				}

				entryPath := rin.Path + "!" + hdr.Name
				res, err := sjob.Call(r, &FileInput{
					Path:   entryPath,
					reader: tr,
				})
				result.Collect(entryPath, res, err)
			}
			return result.Result()
		}

		// .gz (single stream)
		// .tar.gz は上で先に処理済み
		if isGzipPath(rin.Path) {
			gzr, err := gzip.NewReader(source)
			if err != nil {
				return nil, err
			}
			defer gzr.Close()

			innerPath := stripGzipSuffix(rin.Path)
			res, err := sjob.Call(r, &FileInput{
				Path:   innerPath,
				reader: gzr,
			})
			result.Collect(innerPath, res, err)
			return result.Result()
		}

		// normal file / stream
		res, err := handler(r, rin.Path, source)
		result.CollectOne(rin.Path, res, err)
		return result.Result()
	}

	sjob = NewTypedHandler(
		HandlerOption{
			Internal: true,
			Name:     opt.Name + "-stream",
			Version:  opt.Version,
			JobMode:  "cache",
		},
		jobhandler,
	)

	djob = NewTypedHandler(
		HandlerOption{
			Internal: true,
			Name:     opt.Name + "-file",
			Version:  opt.Version,
			JobMode:  "dispatch",
		},
		jobhandler,
	)

	th := NewTypedHandler(opt, func(r *Request, fi *FileInput) (map[string]U, error) {
		result := &ueResult[U]{}

		walkErr := filepath.WalkDir(fi.Path, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				result.CollectOne(path, zeroU, err)
				return nil
			}
			if d == nil || d.IsDir() {
				return nil
			}

			u, err := djob.Call(r, &FileInput{
				Path: path,
			})
			result.Collect(path, u, err)
			return nil
		})
		if walkErr != nil {
			result.CollectOne(fi.Path, zeroU, walkErr)
		}

		return result.Result()
	})

	return th
}

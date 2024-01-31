package ewrap

import (
	"crypto/md5"
	"embed"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

var (
	_ fs.FS = (*embedFSWrapper)(nil)
)

type embedFSWrapper struct {
	fs interface {
		fs.ReadDirFS
		fs.ReadFileFS
	}
	urlPathMap map[string]bool
	md5Hash    string
}

func (efw *embedFSWrapper) Open(name string) (fs.File, error) {
	return efw.fs.Open(name)
}
func (efw *embedFSWrapper) ReadDir(name string) ([]fs.DirEntry, error) {
	return efw.fs.ReadDir(name)
}
func (efw *embedFSWrapper) ReadFile(name string) ([]byte, error) {
	return efw.fs.ReadFile(name)
}
func (efw *embedFSWrapper) Walk(root string, fn fs.WalkDirFunc) error {
	return fs.WalkDir(efw.fs, root, fn)
}

// if path not exist, return a error
func (efw *embedFSWrapper) IsDir(path string) (bool, error) {
	path = "/" + strings.TrimLeft(path, "/")
	isDir, exist := efw.urlPathMap[path]
	if !exist {
		return false, errors.New("not exist")
	} else {
		return isDir, nil
	}
}

// urlPrefix default is "/", useETag default is true
func (efw *embedFSWrapper) FileServer(opt func(notFound *http.HandlerFunc, urlPrefix *string, useETag *bool)) http.HandlerFunc {
	var (
		useETag   = new(bool)
		urlPrefix = new(string)
		notFound  = new(http.HandlerFunc)
	)
	*useETag = true
	*urlPrefix = "/"
	if opt != nil {
		opt(notFound, urlPrefix, useETag)
	}
	*urlPrefix = strings.TrimRight(*urlPrefix, "/")
	fileServer := http.FileServer(http.FS(efw))
	eTag := ""
	if *useETag && efw.md5Hash != "" {
		eTag = "\"" + efw.md5Hash + "\""
	}
	if *notFound == nil {
		*notFound = http.NotFound
	}
	return func(res http.ResponseWriter, req *http.Request) {
		if *urlPrefix != "/" && *urlPrefix != "" {
			if !strings.HasPrefix(req.URL.Path, *urlPrefix) {
				(*notFound)(res, req)
				return
			}
			p := strings.TrimPrefix(req.URL.Path, *urlPrefix)
			rp := strings.TrimPrefix(req.URL.RawPath, *urlPrefix)
			r2 := new(http.Request)
			*r2 = *req
			r2.URL = new(url.URL)
			*r2.URL = *req.URL
			req = r2
			req.URL.Path = p
			req.URL.RawPath = rp
		}
		isDir, err := efw.IsDir(req.URL.Path)
		if err == nil {
			if isDir {
				p := strings.TrimSuffix(req.URL.Path, "/")
				p += "/index.html"
				if _, existIndexHTML := efw.urlPathMap[p]; existIndexHTML {
					if eTag != "" {
						res.Header().Set("ETag", eTag)
					}
					fileServer.ServeHTTP(res, req)
				} else {
					(*notFound)(res, req)
				}
			} else {
				if eTag != "" {
					res.Header().Set("ETag", eTag)
				}
				fileServer.ServeHTTP(res, req)
			}
		} else {
			(*notFound)(res, req)
		}
	}
}

func New(fileSystem embed.FS, subDir ...string) *embedFSWrapper {
	efw := &embedFSWrapper{
		fs:         fileSystem,
		urlPathMap: make(map[string]bool),
	}
	if len(subDir) > 0 && subDir[0] != "" {
		subFS, err := fs.Sub(efw.fs, subDir[0])
		if err == nil {
			efw.fs = (subFS).(interface {
				fs.ReadDirFS
				fs.ReadFileFS
			})
		}
	}
	hasher := md5.New()
	efw.Walk(".", func(path string, d fs.DirEntry, err error) error {
		if path == "." {
			efw.urlPathMap["/"] = true
			return nil
		}
		urlPath := "/" + path
		if d.IsDir() {
			efw.urlPathMap[urlPath] = true
			urlPath += "/"
			efw.urlPathMap[urlPath] = true
		} else {
			f, _ := efw.Open(path)
			io.Copy(hasher, f)
			f.Close()
			efw.urlPathMap[urlPath] = false
		}
		return nil
	})
	efw.md5Hash = hex.EncodeToString(hasher.Sum(nil))
	return efw
}

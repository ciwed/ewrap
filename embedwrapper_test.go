package ewrap

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"
)

//go:embed *
var testFS embed.FS

func TestWrapper(t *testing.T) {
	efw := New(testFS)
	t.Log(efw.urlPathMap)
	isDir, err := efw.IsDir("embedwrapper.go")
	if err != nil {
		t.Fatal("should be nil")
	}
	if isDir {
		t.Fatal("should be a file")
	}
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test/embedwrapper.go", nil)
	efw.FileServer(func(notFound *http.HandlerFunc, prefix *string, useETag *bool) {
		*prefix = "/test/"
		*useETag = true
	}).ServeHTTP(rr, req)
	t.Log(rr.Code)
	if rr.Code != http.StatusOK {
		t.Fatal("should be 200 OK")
	}
	if rr.Header().Get("ETag") == "" {
		t.Fatal("ETag should not be empty")
	}
}

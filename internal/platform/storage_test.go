package platform

import (
	"context"
	"github.com/baidubce/bce-sdk-go/services/bos"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBOSUsesSignedRequestsAndRoundTripsFile(t *testing.T) {
	var stored []byte
	var requestPath string
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "bce-auth-v1/") {
			t.Error("request is not signed")
		}
		if r.Method == "PUT" {
			requestPath = r.URL.Path
			stored, _ = io.ReadAll(r.Body)
			w.Header().Set("ETag", "test-etag")
			w.WriteHeader(200)
			return
		}
		if r.Method == "GET" {
			if r.URL.Path != requestPath {
				t.Error("object key changed")
			}
			w.Header().Set("Content-Type", "image/png")
			w.Write(stored)
			return
		}
		http.Error(w, "unexpected method", 400)
	}))
	defer endpoint.Close()
	client, e := bos.NewClientWithConfig(&bos.BosClientConfiguration{Ak: "test-ak", Sk: "test-sk", Endpoint: endpoint.URL, PathStyleEnable: true, RedirectDisabled: true, ExclusiveHTTPClient: true})
	if e != nil {
		t.Fatal(e)
	}
	storage := &BOSStore{Client: client, Bucket: "tooldeck", Prefix: "tooldeck"}
	src := filepath.Join(t.TempDir(), "test.png")
	os.WriteFile(src, []byte("test-image-content"), 0600)
	file := File{ID: "file_test", Name: "test.png", MIME: "image/png", Size: 18}
	if e = storage.put(context.Background(), &file, src); e != nil {
		t.Fatal(e)
	}
	if file.Storage != "bos" || file.ObjectKey != "tooldeck/files/file_test" {
		t.Fatal(file)
	}
	body, e := storage.open(context.Background(), file)
	if e != nil {
		t.Fatal(e)
	}
	defer body.Close()
	got, _ := io.ReadAll(body)
	if string(got) != "test-image-content" {
		t.Fatal(string(got))
	}
}
func TestBOSConfigurationFailsClosed(t *testing.T) {
	t.Setenv("TOOLDECK_BOS_ENDPOINT", "https://bj.bcebos.com")
	t.Setenv("TOOLDECK_BOS_BUCKET", "tooldeck")
	t.Setenv("TOOLDECK_BOS_AK", "")
	t.Setenv("TOOLDECK_BOS_SK", "")
	if _, e := newBOSStore(); e == nil {
		t.Fatal("accepted missing credentials")
	}
}

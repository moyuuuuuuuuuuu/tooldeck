package platform

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/baidubce/bce-sdk-go/services/bos"
	"github.com/baidubce/bce-sdk-go/services/bos/api"
)

// Object storage is accessed only by the control plane; BOS credentials are never injected into tools.
type BOSStore struct {
	Client         *bos.Client
	Bucket, Prefix string
}

func newBOSStore() (*BOSStore, error) {
	endpoint := os.Getenv("TOOLDECK_BOS_ENDPOINT")
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" {
		return nil, errors.New("BOS endpoint must be an HTTPS service endpoint")
	}
	bucket := os.Getenv("TOOLDECK_BOS_BUCKET")
	ak := os.Getenv("TOOLDECK_BOS_AK")
	sk := os.Getenv("TOOLDECK_BOS_SK")
	if bucket == "" || ak == "" || sk == "" {
		return nil, errors.New("BOS bucket, AK and SK are required")
	}
	prefix := strings.Trim(os.Getenv("TOOLDECK_BOS_PREFIX"), "/")
	if prefix == "" {
		prefix = "tooldeck"
	}
	if !validPath(prefix) {
		return nil, errors.New("invalid BOS prefix")
	}
	timeout := 60 * time.Second
	client, e := bos.NewClientWithConfig(&bos.BosClientConfiguration{Ak: ak, Sk: sk, Endpoint: endpoint, RedirectDisabled: true, ExclusiveHTTPClient: true, HTTPClientTimeout: &timeout})
	if e != nil {
		return nil, e
	}
	return &BOSStore{client, bucket, prefix}, nil
}
func (b *BOSStore) put(ctx context.Context, f *File, path string) error {
	key := b.Prefix + "/files/" + f.ID
	_, e := b.Client.PutObjectFromFileWithContext(ctx, b.Bucket, key, path, &api.PutObjectArgs{ContentType: f.MIME, CacheControl: "private, no-store", StorageClass: api.STORAGE_CLASS_STANDARD})
	if e != nil {
		return e
	}
	f.Storage = "bos"
	f.ObjectKey = key
	return nil
}
func (b *BOSStore) open(ctx context.Context, f File) (io.ReadCloser, error) {
	result, e := b.Client.GetObjectWithContext(ctx, b.Bucket, f.ObjectKey, nil)
	if e != nil {
		return nil, e
	}
	return result.Body, nil
}
func (s *Server) persistFile(ctx context.Context, f *File, source string) error {
	if s.bos != nil {
		return s.bos.put(ctx, f, source)
	}
	f.Storage = "local"
	dest := filepath.Join(s.store.Root, "files", f.ID)
	if filepath.Clean(source) == dest {
		return nil
	}
	return copyFile(source, dest)
}
func (s *Server) openFile(ctx context.Context, f File) (io.ReadCloser, error) {
	if f.Storage == "bos" {
		if s.bos == nil {
			return nil, errors.New("BOS storage is not configured")
		}
		return s.bos.open(ctx, f)
	}
	return os.Open(filepath.Join(s.store.Root, "files", f.ID))
}
func (s *Server) fetchFile(ctx context.Context, f File, dest string) error {
	src, e := s.openFile(ctx, f)
	if e != nil {
		return e
	}
	defer src.Close()
	out, e := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0444)
	if e != nil {
		return e
	}
	n, e := io.Copy(out, io.LimitReader(src, f.Size+1))
	ce := out.Close()
	if e == nil {
		e = ce
	}
	if n != f.Size && e == nil {
		e = errors.New("stored file size mismatch")
	}
	return e
}

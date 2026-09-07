package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	artifactLifetime  = 7 * 24 * time.Hour
	artifactDownloads = 3
)

func (s *Server) artifactSignature(id string, expires int64) string {
	mac := hmac.New(sha256.New, []byte(os.Getenv("TOOLDECK_MASTER_KEY")))
	fmt.Fprintf(mac, "%s\n%d", id, expires)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) artifactURL(f File) string {
	base := strings.TrimRight(os.Getenv("TOOLDECK_PUBLIC_URL"), "/")
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || f.RunID == "" || f.ExpiresAt.IsZero() {
		return ""
	}
	expires := f.ExpiresAt.Unix()
	return base + "/api/public/artifacts/" + url.PathEscape(f.ID) + "?exp=" + strconv.FormatInt(expires, 10) + "&sig=" + url.QueryEscape(s.artifactSignature(f.ID, expires))
}

func (s *Server) downloadSignedArtifact(w http.ResponseWriter, r *http.Request, id string) {
	expires, err := strconv.ParseInt(r.URL.Query().Get("exp"), 10, 64)
	signature := r.URL.Query().Get("sig")
	expected := s.artifactSignature(id, expires)
	if err != nil || expires <= time.Now().Unix() || !hmac.Equal([]byte(signature), []byte(expected)) {
		fail(w, http.StatusGone, "下载链接无效或已过期")
		return
	}
	s.store.Lock()
	f, ok := s.store.State.Files[id]
	s.store.Unlock()
	if !ok || f.RunID == "" {
		fail(w, http.StatusGone, "运行产物已删除")
		return
	}
	s.serveRunArtifact(w, r, f)
}

func (s *Server) serveRunArtifact(w http.ResponseWriter, r *http.Request, f File) {
	now := time.Now()
	s.store.Lock()
	current, ok := s.store.State.Files[f.ID]
	if !ok || current.RunID == "" || !current.ExpiresAt.After(now) || current.DownloadsRemaining <= 0 {
		s.store.Unlock()
		fail(w, http.StatusGone, "运行产物已过期或下载次数已用完")
		return
	}
	current.DownloadsRemaining--
	s.store.State.Files[current.ID] = current
	s.updateRunArtifactLocked(current)
	if err := s.store.save(); err != nil {
		s.store.Unlock()
		fail(w, 500, "cannot reserve artifact download")
		return
	}
	s.store.Unlock()

	body, err := s.openFile(r.Context(), current)
	if err != nil {
		s.store.Lock()
		if restored, exists := s.store.State.Files[current.ID]; exists {
			restored.DownloadsRemaining++
			s.store.State.Files[current.ID] = restored
			s.updateRunArtifactLocked(restored)
			_ = s.store.save()
		}
		s.store.Unlock()
		fail(w, 502, "object storage download failed")
		return
	}
	defer body.Close()
	s.writeDownloadHeaders(w, current)
	_, _ = io.Copy(w, body)
	if current.DownloadsRemaining == 0 {
		s.removeArtifact(current)
	}
}

func (s *Server) updateRunArtifactLocked(file File) {
	run := s.store.State.Runs[file.RunID]
	for i := range run.Artifacts {
		if run.Artifacts[i].ID == file.ID {
			run.Artifacts[i] = file
		}
	}
	s.store.State.Runs[run.ID] = run
}

func (s *Server) removeArtifact(file File) {
	if err := s.deleteStoredFile(file); err != nil {
		return
	}
	s.store.Lock()
	defer s.store.Unlock()
	delete(s.store.State.Files, file.ID)
	delete(s.store.State.Owners, file.ID)
	run := s.store.State.Runs[file.RunID]
	kept := run.Artifacts[:0]
	for _, item := range run.Artifacts {
		if item.ID != file.ID {
			kept = append(kept, item)
		}
	}
	run.Artifacts = kept
	s.store.State.Runs[run.ID] = run
	_ = s.store.save()
}

func (s *Server) artifactCleanupWorker(ctx context.Context) {
	s.initializeArtifactPolicies()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		s.cleanupExpiredArtifacts()
	}
}

func (s *Server) initializeArtifactPolicies() {
	s.store.Lock()
	changed := false
	for id, file := range s.store.State.Files {
		if file.RunID == "" || !file.ExpiresAt.IsZero() {
			continue
		}
		run := s.store.State.Runs[file.RunID]
		file.ExpiresAt = run.Created.Add(artifactLifetime)
		file.DownloadsRemaining = artifactDownloads
		s.store.State.Files[id] = file
		s.updateRunArtifactLocked(file)
		changed = true
	}
	if changed {
		_ = s.store.save()
	}
	s.store.Unlock()
	s.cleanupExpiredArtifacts()
}

func (s *Server) cleanupExpiredArtifacts() {
	now := time.Now()
	s.store.Lock()
	var expired []File
	for _, file := range s.store.State.Files {
		if file.RunID != "" && (!file.ExpiresAt.After(now) || file.DownloadsRemaining <= 0) {
			expired = append(expired, file)
		}
	}
	s.store.Unlock()
	for _, file := range expired {
		s.removeArtifact(file)
	}
}

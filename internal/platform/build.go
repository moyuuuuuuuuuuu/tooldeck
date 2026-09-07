package platform

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (s *Server) buildEndpoint(w http.ResponseWriter, r *http.Request, p Principal, id string) {
	if !p.Session {
		fail(w, 403, "user session required")
		return
	}
	s.store.Lock()
	defer s.store.Unlock()
	t, ok := s.store.State.Tools[id]
	if !ok || (!p.Admin && toolOwner(t) != p.owner()) {
		fail(w, 404, "tool not found")
		return
	}
	if r.Method == "POST" {
		if t.BuildStatus != "pending" && t.BuildStatus != "failed" {
			fail(w, 409, "仅待构建或失败的版本可以发起构建")
			return
		}
		old := t
		t.BuildStatus = "queued"
		t.BuildError = ""
		t.BuildLog = ""
		s.store.State.Tools[id] = t
		if e := s.store.save(); e != nil {
			s.store.State.Tools[id] = old
			fail(w, 500, e)
			return
		}
	} else if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	jsonResponse(w, 200, t)
}
func (s *Server) buildWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		s.store.Lock()
		var t Tool
		for _, item := range s.store.State.Tools {
			if item.BuildStatus == "queued" && (t.ID == "" || item.Created.Before(t.Created)) {
				t = item
			}
		}
		if t.ID != "" {
			old := t
			t.BuildStatus = "building"
			s.store.State.Tools[t.ID] = t
			if e := s.store.save(); e != nil {
				s.store.State.Tools[t.ID] = old
				t.ID = ""
			}
		}
		s.store.Unlock()
		if t.ID != "" {
			s.buildTool(ctx, t)
		}
	}
}
func (s *Server) buildTool(parent context.Context, t Tool) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	artifact := ID("build_")
	dest := filepath.Join(s.store.Root, "artifacts", artifact)
	job := filepath.Join(s.store.Root, "jobs", artifact)
	var buildErr error
	var image string
	logs := &cappedBuffer{Limit: 1 << 20}
	defer func() {
		_ = os.RemoveAll(job)
		s.store.Lock()
		defer s.store.Unlock()
		current := s.store.State.Tools[t.ID]
		current.BuildLog = logs.String()
		if buildErr != nil {
			current.BuildStatus = "failed"
			current.BuildError = buildErr.Error()
			_ = os.RemoveAll(dest)
		} else {
			current.BuildStatus = "ready"
			current.BuildError = ""
			current.Artifact = artifact
			current.BuildImage = image
		}
		s.store.State.Tools[t.ID] = current
		if e := s.store.save(); e != nil {
			fmt.Fprintln(os.Stderr, "build save failed:", e)
		}
	}()
	runtime := t.Manifest.Runtime
	switch runtime {
	case "js":
		runtime = "node"
	case "py":
		runtime = "python"
	case "golang":
		runtime = "go"
	}
	b, e := exec.CommandContext(ctx, "docker", "image", "inspect", "tooldeck-runtime-"+runtime+":"+t.Manifest.RuntimeVersion+"-build2", "--format", "{{.Id}}").Output()
	if e != nil {
		version, versionErr := runtimeVersion(t.Manifest)
		if versionErr != nil {
			buildErr = versionErr
			return
		}
		base := runtime + ":" + version
		switch runtime {
		case "node":
			base += "-bookworm-slim"
		case "python":
			base += "-slim-bookworm"
		case "php":
			if version == "8.0" || version == "8.1" {
				base += "-cli-bullseye"
			} else {
				base += "-cli-bookworm"
			}
		case "go":
			base = "golang:" + version + "-bookworm"
		}
		logs.Write([]byte("首次使用该版本，正在准备运行环境...\n"))
		prepareCtx, prepareCancel := context.WithTimeout(parent, 20*time.Minute)
		tag := "tooldeck-runtime-" + runtime + ":" + version + "-build2"
		prepare := exec.CommandContext(prepareCtx, "docker", "build", "--progress=plain", "--build-arg", "BASE_IMAGE="+base, "-t", tag, "-f", "/runtime-context/runtimes/"+runtime+".Dockerfile", "/runtime-context")
		prepare.Stdout = logs
		prepare.Stderr = logs
		prepareErr := prepare.Run()
		prepareCancel()
		if prepareErr != nil {
			buildErr = errors.New("运行环境准备失败，请查看日志后重试")
			return
		}
		cancel()
		ctx, cancel = context.WithTimeout(parent, 10*time.Minute)
		defer cancel()
		b, e = exec.CommandContext(ctx, "docker", "image", "inspect", tag, "--format", "{{.Id}}").Output()
		if e != nil {
			buildErr = e
			return
		}
	}
	image = strings.TrimSpace(string(b))
	if e = os.MkdirAll(job, 0755); e != nil {
		buildErr = e
		return
	}
	listener, e := net.Listen("unix", filepath.Join(job, "proxy.sock"))
	if e != nil {
		buildErr = e
		return
	}
	_ = os.Chmod(filepath.Join(job, "proxy.sock"), 0666)
	proxy := &http.Server{Handler: egressProxy([]string{"registry.npmjs.org", "repo.packagist.org", "packagist.org", "api.github.com", "codeload.github.com", "github.com", "objects.githubusercontent.com", "pypi.org", "files.pythonhosted.org", "proxy.golang.org", "sum.golang.org", "storage.googleapis.com"}), ReadHeaderTimeout: 10 * time.Second}
	go proxy.Serve(listener)
	defer proxy.Close()
	name := "tooldeck-build-" + artifact
	args := []string{"run", "--name", name, "--network", "none", "--read-only", "--user", "65534:65534", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--pids-limit", "256", "--cpus", "1", "--memory", "1g", "--memory-swap", "1g", "--tmpfs", "/build:rw,exec,nosuid,nodev,size=536870912,mode=1777", "--tmpfs", "/tmp:rw,exec,nosuid,nodev,size=268435456,mode=1777", "--mount", "type=bind,src=" + filepath.Join(s.hostData, "packages", t.ID) + ",dst=/source,readonly", "--mount", "type=bind,src=" + filepath.Join(s.hostData, "jobs", artifact) + ",dst=/job,readonly", "--env", "HOME=/tmp", "--env", "GOCACHE=/tmp/go-cache", "--env", "GOMODCACHE=/tmp/go-mod", "--entrypoint", "/build.sh", image, t.Manifest.Entrypoint, t.Manifest.BuildCommand}
	args = sandboxArgs(args)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stderr = logs
	pipe, e := cmd.StdoutPipe()
	if e != nil {
		buildErr = e
		return
	}
	if e = cmd.Start(); e != nil {
		buildErr = e
		return
	}
	unpackErr := extractBuild(pipe, dest)
	// Drain bounded tar padding before Wait: small host pipes can otherwise block the Docker client.
	if unpackErr == nil {
		n, drainErr := io.Copy(io.Discard, io.LimitReader(pipe, 1<<20))
		if drainErr != nil {
			unpackErr = drainErr
		} else if n == 1<<20 {
			unpackErr = errors.New("excessive trailing build output")
		}
	}
	if unpackErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	cleanup, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	_ = exec.CommandContext(cleanup, "docker", "rm", "-f", name).Run()
	if unpackErr != nil {
		buildErr = unpackErr
		return
	}
	if waitErr != nil {
		buildErr = fmt.Errorf("构建失败，请查看日志：%v", waitErr)
		return
	}
	entry, e := os.Lstat(filepath.Join(dest, t.Manifest.Entrypoint))
	if e != nil || !entry.Mode().IsRegular() {
		buildErr = errors.New("构建产物缺少入口文件")
		return
	}
	if runtime == "go" {
		f, e := os.Lstat(filepath.Join(dest, ".tooldeck-bin"))
		if e != nil || !f.Mode().IsRegular() {
			buildErr = errors.New("Go 构建未生成可执行文件")
			return
		}
	}
}

// Extract bounded build output without following archive-controlled symlinks.
func extractBuild(reader io.Reader, dest string) error {
	if e := os.MkdirAll(dest, 0755); e != nil {
		return e
	}
	r := tar.NewReader(io.LimitReader(reader, 600<<20))
	var total int64
	count := 0
	for {
		h, e := r.Next()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		name := strings.TrimPrefix(h.Name, "./")
		name = strings.TrimSuffix(name, "/")
		if name == "" || name == "." {
			continue
		}
		count++
		total += h.Size
		if !validPath(name) || count > 40000 || h.Size < 0 || total > 512<<20 {
			return errors.New("构建产物路径或容量超限")
		}
		path := filepath.Join(dest, name)
		parent := filepath.Dir(path)
		for p := parent; p != dest; p = filepath.Dir(p) {
			st, e := os.Lstat(p)
			if e == nil && !st.IsDir() {
				return errors.New("unsafe artifact parent")
			}
		}
		if _, e := os.Lstat(path); e == nil {
			if h.Typeflag == tar.TypeDir {
				st, _ := os.Lstat(path)
				if st.IsDir() {
					continue
				}
			}
			return errors.New("duplicate build artifact")
		}
		if e = os.MkdirAll(parent, 0755); e != nil {
			return e
		}
		switch h.Typeflag {
		case tar.TypeDir:
			e = os.MkdirAll(path, 0755)
		case tar.TypeReg:
			var f *os.File
			f, e = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(h.Mode)&0755|0400)
			if e == nil {
				_, e = io.Copy(f, r)
				f.Close()
			}
		case tar.TypeSymlink:
			target := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(name), h.Linkname)))
			if strings.HasPrefix(h.Linkname, "/") || strings.ContainsAny(h.Linkname, "\\:") || !validPath(target) {
				return errors.New("unsafe artifact symlink")
			}
			e = os.Symlink(h.Linkname, path)
		default:
			return errors.New("unsupported artifact entry")
		}
		if e != nil {
			return e
		}
	}
}

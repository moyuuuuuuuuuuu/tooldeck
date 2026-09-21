package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type DockerResource struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Candidate bool   `json:"candidate"`
}

func (s *Server) dockerResources(ctx context.Context) ([]DockerResource, error) {
	list := []DockerResource{}
	filter := "label=tooldeck.instance=" + s.store.State.InstanceID
	builds := false
	protected := map[string]bool{}
	for _, t := range s.store.State.Tools {
		protected[t.BuildImage] = true
		if t.BuildStatus == "building" {
			builds = true
		}
	}
	for _, kind := range []string{"container", "image"} {
		args := []string{kind, "ls", "--filter", filter, "--quiet", "--no-trunc"}
		if kind == "container" {
			args = append(args, "--all")
		}
		out, err := exec.CommandContext(ctx, "docker", args...).Output()
		if err != nil {
			return nil, errors.New("Docker 资源扫描失败，请检查节点连接")
		}
		seen := map[string]bool{}
		for _, id := range strings.Fields(string(out)) {
			if seen[id] {
				continue
			}
			seen[id] = true
			raw, err := exec.CommandContext(ctx, "docker", kind, "inspect", id).Output()
			if err != nil {
				return nil, err
			}
			var objects []struct {
				ID       string `json:"Id"`
				Name     string
				Created  time.Time
				RepoTags []string
				Config   struct{ Labels map[string]string }
				State    struct{ Running bool }
			}
			if err = json.Unmarshal(raw, &objects); err != nil || len(objects) != 1 {
				return nil, errors.New("invalid Docker resource response")
			}
			obj := objects[0]
			if obj.Config.Labels["tooldeck.instance"] != s.store.State.InstanceID {
				continue
			}
			name := strings.TrimPrefix(obj.Name, "/")
			candidate := time.Since(obj.Created) > 24*time.Hour && !builds
			if kind == "image" {
				name = strings.Join(obj.RepoTags, ", ")
				candidate = candidate && !protected[obj.ID]
			} else {
				runID := strings.TrimPrefix(name, "tooldeck-run-")
				run := s.store.State.Runs[runID]
				_, executing := s.activeRuns[runID]
				candidate = candidate && !runActive(run.Status) && !executing && !obj.State.Running
			}
			list = append(list, DockerResource{obj.ID, kind, name, candidate})
		}
	}
	return list, nil
}
func (s *Server) dockerStorageEndpoint(w http.ResponseWriter, r *http.Request, p Principal) {
	if !p.Admin {
		fail(w, 403, "admin session required")
		return
	}
	var b struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}
	if r.Method == "POST" {
		if err := decode(w, r, &b); err != nil {
			fail(w, 400, err)
			return
		}
	} else if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	s.store.Lock()
	defer s.store.Unlock()
	list, err := s.dockerResources(ctx)
	if err != nil {
		fail(w, 503, err)
		return
	}
	if r.Method == "GET" {
		jsonResponse(w, 200, list)
		return
	}
	for _, item := range list {
		if item.ID == b.ID && item.Kind == b.Kind && item.Candidate {
			// Never force-remove images or containers. Docker's reference checks remain
			// the final guard, including containers belonging to other applications.
			if err = exec.CommandContext(ctx, "docker", item.Kind, "rm", item.ID).Run(); err != nil {
				fail(w, 409, "资源仍被 Docker 引用，请刷新后重试")
				return
			}
			jsonResponse(w, 200, true)
			return
		}
	}
	fail(w, 409, "资源不在可清理列表或仍被引用")
}

// Before starting either worker pool, stop containers left behind by the same
// instance. Startup recovery deliberately fails interrupted jobs rather than
// replaying their external side effects. New work must not overlap their old
// containers. Shared and legacy unlabelled containers are never touched.
func (s *Server) reconcileContainers(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "docker", "container", "ls", "--all", "--quiet", "--no-trunc", "--filter", "label=tooldeck.instance="+s.store.State.InstanceID).Output()
	if err != nil {
		return err
	}
	for _, id := range strings.Fields(string(output)) {
		raw, err := exec.CommandContext(ctx, "docker", "container", "inspect", id).Output()
		if err != nil {
			return err
		}
		var list []struct {
			Name   string
			Config struct{ Labels map[string]string }
		}
		if err = json.Unmarshal(raw, &list); err != nil || len(list) != 1 {
			return errors.New("invalid container response")
		}
		item := list[0]
		name := strings.TrimPrefix(item.Name, "/")
		if item.Config.Labels["tooldeck.instance"] != s.store.State.InstanceID || (!strings.HasPrefix(name, "tooldeck-run-") && !strings.HasPrefix(name, "tooldeck-build-")) {
			continue
		}
		if err = exec.CommandContext(ctx, "docker", "container", "rm", "--force", id).Run(); err != nil {
			return err
		}
	}
	return nil
}

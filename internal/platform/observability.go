package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type AuditEvent struct {
	ID      string    `json:"id"`
	Actor   string    `json:"actor"`
	Action  string    `json:"action"`
	Object  string    `json:"object"`
	IP      string    `json:"ip"`
	Status  int       `json:"status"`
	Error   string    `json:"error,omitempty"`
	Created time.Time `json:"created_at"`
}
type auditKey struct{}
type auditIdentity struct{ actor string }

func auditActor(r *http.Request, actor string) {
	if v, ok := r.Context().Value(auditKey{}).(*auditIdentity); ok {
		v.actor = actor
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(n int) {
	if w.status == 0 {
		w.status = n
		w.ResponseWriter.WriteHeader(n)
	}
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *statusWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (s *Server) auditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" || r.Method == "HEAD" || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		ident := &auditIdentity{actor: "anonymous"}
		r = r.WithContext(context.WithValue(r.Context(), auditKey{}, ident))
		recorder := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		status := recorder.status
		if status == 0 {
			status = 200
		}
		// Only route identifiers are recorded. Never record body, query, headers,
		// provider errors, tokens, passwords or environment values.
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		action := r.Method
		object := ""
		if len(parts) >= 3 {
			action += " " + parts[1] + "/" + parts[2]
		}
		if len(parts) >= 4 && len(parts[3]) <= 100 && namePattern.MatchString(parts[3]) {
			object = parts[3]
		}
		if len(parts) >= 5 && len(parts[4]) <= 40 && namePattern.MatchString(parts[4]) {
			action += "/" + parts[4]
		}
		event := AuditEvent{ID: ID("audit_"), Actor: ident.actor, Action: action, Object: object, IP: ip, Status: status, Created: time.Now()}
		if status >= 400 {
			event.Error = http.StatusText(status)
		}
		s.store.Lock()
		s.store.State.Audit[event.ID] = event
		if len(s.store.State.Audit) > 10000 {
			var oldest AuditEvent
			for _, v := range s.store.State.Audit {
				if oldest.ID == "" || v.Created.Before(oldest.Created) {
					oldest = v
				}
			}
			delete(s.store.State.Audit, oldest.ID)
		}
		err = s.store.save()
		s.store.Unlock()
		logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
		logger.Info("audit", "actor", event.Actor, "action", event.Action, "object", event.Object, "status", status, "ip", ip)
		if err != nil {
			logger.Error("audit persistence failed", "event_id", event.ID)
		}
	})
}
func (s *Server) auditEndpoint(w http.ResponseWriter, r *http.Request, p Principal) {
	if !p.Admin {
		fail(w, 403, "admin session required")
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	s.store.Lock()
	list := []AuditEvent{}
	for _, e := range s.store.State.Audit {
		if q := r.URL.Query().Get("actor"); q == "" || q == e.Actor {
			list = append(list, e)
		}
	}
	s.store.Unlock()
	sort.Slice(list, func(i, j int) bool { return list[i].Created.After(list[j].Created) })
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	a, b := pageBounds(len(list), page, 50)
	jsonResponse(w, 200, map[string]any{"items": list[a:b], "total": len(list), "page": page})
}
func (s *Server) metricsSnapshot() map[string]any {
	result := s.queueMetrics()
	if total, free, err := diskCapacity(s.store.Root); err == nil {
		result["disk_total_bytes"] = total
		result["disk_free_bytes"] = free
	}
	s.store.Lock()
	defer s.store.Unlock()
	complete, success, buildDone, buildFailed, callbacks, callbackFailed := 0, 0, 0, 0, 0, 0
	durations := []int64{}
	for _, r := range s.store.State.Runs {
		if terminalRun(r.Status) {
			complete++
			if r.Status == "succeeded" {
				success++
			}
			durations = append(durations, r.Duration)
		}
		if r.CallbackSent || r.CallbackFailed {
			callbacks++
			if r.CallbackFailed {
				callbackFailed++
			}
		}
	}
	for _, t := range s.store.State.Tools {
		if t.BuildStatus == "ready" || t.BuildStatus == "failed" {
			buildDone++
			if t.BuildStatus == "failed" {
				buildFailed++
			}
		}
	}
	ratio := func(n, d int) float64 {
		if d == 0 {
			return 0
		}
		return float64(n) / float64(d)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := int64(0)
	if len(durations) > 0 {
		p95 = durations[(len(durations)*95+99)/100-1]
	}
	result["completed"] = complete
	result["success_rate"] = ratio(success, complete)
	result["p95_duration_ms"] = p95
	result["build_failure_rate"] = ratio(buildFailed, buildDone)
	result["callback_failure_rate"] = ratio(callbackFailed, callbacks)
	result["worker_concurrency"] = s.queue.Workers
	result["user_concurrency"] = s.queue.PerUser
	result["build_concurrency"] = s.queue.Builds
	return result
}
func (s *Server) metricsEndpoint(w http.ResponseWriter, r *http.Request, p Principal) {
	if !p.Admin {
		fail(w, 403, "admin session required")
		return
	}
	if r.Method != "GET" {
		fail(w, 405, "method not allowed")
		return
	}
	metrics := s.metricsSnapshot()
	if r.URL.Query().Get("format") == "prometheus" {
		if os.Getenv("TOOLDECK_PROMETHEUS") != "true" {
			fail(w, 404, "Prometheus is disabled")
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		keys := []string{}
		for k := range metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k == "limit_hits" {
				for _, name := range []string{"global", "user", "tool"} {
					fmt.Fprintf(w, "tooldeck_queue_limit_hits_total{limit=%q} %d\n", name, metrics[k].(map[string]uint64)[name])
				}
				continue
			}
			value, _ := json.Marshal(metrics[k])
			fmt.Fprintf(w, "tooldeck_%s %s\n", k, value)
		}
		return
	}
	jsonResponse(w, 200, metrics)
}

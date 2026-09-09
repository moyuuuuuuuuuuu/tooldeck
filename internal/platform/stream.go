package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type StreamEvent struct {
	Text string `json:"text"`
}

const eventPrefix = "TOOLDECK_EVENT "

// stderr events are distinct from final stdout JSON and the artifact archive.
type eventWriter struct {
	pending []byte
	total   int
	logs    *cappedBuffer
	emit    func(StreamEvent)
	err     error
}

func (e *eventWriter) Write(p []byte) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	e.total += len(p)
	if e.total > 3<<20 {
		e.err = errors.New("stream output exceeded limit")
		return 0, e.err
	}
	e.pending = append(e.pending, p...)
	for {
		i := bytes.IndexByte(e.pending, '\n')
		if i < 0 {
			break
		}
		if i > 64<<10 {
			e.err = errors.New("stream event line exceeds 64 KB")
			return 0, e.err
		}
		line := e.pending[:i]
		e.pending = e.pending[i+1:]
		if bytes.HasPrefix(line, []byte(eventPrefix)) {
			var v struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(line[len(eventPrefix):], &v); err != nil || v.Type != "delta" {
				e.err = errors.New("invalid stream event; expected delta with text")
				return 0, e.err
			}
			e.emit(StreamEvent{Text: v.Text})
		} else {
			e.logs.Write(append(append([]byte{}, line...), '\n'))
		}
	}
	if len(e.pending) > 64<<10 {
		e.err = errors.New("stream event line exceeds 64 KB")
		return 0, e.err
	}
	return len(p), nil
}
func (e *eventWriter) finish() error {
	if len(e.pending) > 0 {
		_, err := e.Write([]byte("\n"))
		if err != nil {
			return err
		}
	}
	return e.err
}

var sseSlots = make(chan struct{}, 32)

func (s *Server) streamRun(w http.ResponseWriter, r *http.Request, p Principal, id string) {
	if r.Method != "GET" && r.Method != "POST" {
		fail(w, 405, "method not allowed")
		return
	}
	cursor := 0
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 {
			fail(w, 400, "invalid Last-Event-ID")
			return
		}
		cursor = n
	}
	s.store.Lock()
	run, ok := s.store.State.Runs[id]
	tool := s.store.State.Tools[run.ToolID]
	allowed := ok && s.canReadRun(p, run) && (p.Session || p.Guest || tool.APIEnabled == nil || *tool.APIEnabled)
	s.store.Unlock()
	if !allowed {
		fail(w, 404, "run unavailable")
		return
	}
	if cursor > len(run.Events) {
		fail(w, 409, "event cursor unavailable; reload run")
		return
	}
	select {
	case sseSlots <- struct{}{}:
		defer func() { <-sseSlots }()
	default:
		fail(w, 429, "too many stream connections")
		return
	}
	if _, ok := w.(http.Flusher); !ok {
		fail(w, 500, "streaming unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	rc := http.NewResponseController(w)
	send := func(kind string, data any, eventID int) bool {
		_ = rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		b, _ := json.Marshal(data)
		if eventID > 0 {
			if _, e := fmt.Fprintf(w, "id: %d\n", eventID); e != nil {
				return false
			}
		}
		if _, e := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, b); e != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !send("run", map[string]string{"run_id": id}, 0) {
		return
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	lastStatus := ""
	for {
		s.store.Lock()
		run = s.store.State.Runs[id]
		tool = s.store.State.Tools[run.ToolID]
		allowed = s.canReadRun(p, run) && (p.Session || p.Guest || tool.APIEnabled == nil || *tool.APIEnabled)
		events := append([]StreamEvent(nil), run.Events[cursor:]...)
		s.store.Unlock()
		if !allowed {
			return
		}
		if run.Status != lastStatus {
			lastStatus = run.Status
			if !send("status", map[string]string{"status": run.Status}, 0) {
				return
			}
		}
		for _, e := range events {
			cursor++
			if !send("delta", e, cursor) {
				return
			}
		}
		if !runActive(run.Status) {
			if run.Error != "" {
				if !send("error", map[string]string{"message": run.Error}, 0) {
					return
				}
			}
			run.Events = nil
			if !send("result", run, 0) {
				return
			}
			send("done", map[string]string{"status": run.Status}, 0)
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		case <-heartbeat.C:
			if p.Guest {
				if !send("heartbeat", map[string]string{"run_id": id}, 0) {
					return
				}
				continue
			}
			fresh, err := s.authenticate(r)
			if err != nil {
				return
			}
			p = fresh
			if !send("heartbeat", map[string]string{"run_id": id}, 0) {
				return
			}
		}
	}
}

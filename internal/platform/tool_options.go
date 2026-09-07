package platform

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"
)

func canUseTool(p Principal, t Tool) bool {
	if p.Guest {
		return guestTool(t)
	}
	if !p.allows(t.Manifest.Name) {
		return false
	}
	if p.Admin || t.Owner != "" && t.Owner == p.owner() {
		return true
	}
	return !t.Withdrawn && (t.BuildStatus == "" || t.BuildStatus == "ready") && (t.Public == nil || *t.Public) && (t.ReviewStatus == "" || t.ReviewStatus == "approved")
}

func (s *Server) notifications(w http.ResponseWriter, r *http.Request, p Principal, parts []string) {
	if !p.Session {
		fail(w, 403, "user session required")
		return
	}
	s.store.Lock()
	defer s.store.Unlock()
	if r.Method == "GET" && len(parts) == 1 {
		list := []Run{}
		for _, run := range s.store.State.Runs {
			if run.Owner == p.owner() && !run.NotificationRead && s.store.State.Tools[run.ToolID].Notify && run.Status != "queued" && run.Status != "running" {
				list = append(list, run)
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Created.After(list[j].Created) })
		if len(list) > 100 {
			list = list[:100]
		}
		jsonResponse(w, 200, list)
		return
	}
	if r.Method == "POST" && len(parts) == 2 {
		run, ok := s.store.State.Runs[parts[1]]
		if !ok || run.Owner != p.owner() {
			fail(w, 404, "not found")
			return
		}
		old := run
		run.NotificationRead = true
		s.store.State.Runs[run.ID] = run
		if e := s.store.save(); e != nil {
			s.store.State.Runs[run.ID] = old
			fail(w, 500, e)
			return
		}
		jsonResponse(w, 200, true)
		return
	}
	fail(w, 405, "method not allowed")
}

func (s *Server) notificationWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if os.Getenv("TOOLDECK_SMTP_FROM") == "" {
			continue
		}
		s.store.Lock()
		var selected Run
		var recipient string
		var title string
		for _, run := range s.store.State.Runs {
			tool := s.store.State.Tools[run.ToolID]
			user := s.store.State.Users[run.Owner]
			if tool.Notify && user.EmailVerified && user.Email != "" && !run.EmailSent && run.EmailAttempts < 3 && !time.Now().Before(run.EmailNext) && run.Status != "queued" && run.Status != "running" {
				selected = run
				recipient = user.Email
				title = tool.Manifest.Title
				break
			}
		}
		if selected.ID == "" {
			s.store.Unlock()
			continue
		}
		old := selected
		selected.EmailAttempts++
		selected.EmailNext = time.Now().Add(time.Minute * time.Duration(selected.EmailAttempts))
		s.store.State.Runs[selected.ID] = selected
		if e := s.store.save(); e != nil {
			s.store.State.Runs[selected.ID] = old
			s.store.Unlock()
			continue
		}
		s.store.Unlock()
		// Send a completion notice, not potentially sensitive input, output or log contents.
		e := sendSMTPMessage(recipient, "ToolDeck task completed", fmt.Sprintf("工具：%s\r\n任务：%s\r\n状态：%s\r\n请登录 ToolDeck，在“我的记录”查看执行结果。", title, selected.ID, selected.Status))
		if e == nil {
			s.store.Lock()
			run := s.store.State.Runs[selected.ID]
			run.EmailSent = true
			s.store.State.Runs[selected.ID] = run
			_ = s.store.save()
			s.store.Unlock()
		}
	}
}

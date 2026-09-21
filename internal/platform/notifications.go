package platform

import (
	"context"
	"net/http"
	"sort"
	"time"
)

type Notice struct {
	ID          string    `json:"id"`
	User        string    `json:"owner"`
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`
	Object      string    `json:"object"`
	Read        bool      `json:"read"`
	Created     time.Time `json:"created_at"`
	EmailStatus string    `json:"email_status,omitempty"`
	Attempts    int       `json:"email_attempts,omitempty"`
	Next        time.Time `json:"email_next,omitempty"`
}
type NoticePreferences struct {
	InApp bool `json:"in_app"`
	Email bool `json:"email"`
}

func (s *Store) notice(owner, kind, object, title string) {
	if _, ok := s.State.Users[owner]; !ok {
		return
	}
	prefs, exists := s.State.NoticePreferences[owner]
	if !exists {
		prefs.InApp = true
	}
	if !prefs.InApp && !prefs.Email {
		return
	}
	id := ID("notice_")
	status := ""
	if prefs.Email {
		status = "pending"
	}
	s.State.Notices[id] = Notice{ID: id, User: owner, Kind: kind, Title: title, Object: object, Read: !prefs.InApp, Created: time.Now(), EmailStatus: status}
}
func (s *Server) inboxEndpoint(w http.ResponseWriter, r *http.Request, p Principal, parts []string) {
	if !p.Session {
		fail(w, 403, "user session required")
		return
	}
	s.store.Lock()
	defer s.store.Unlock()
	if len(parts) == 2 && parts[1] == "preferences" {
		prefs, ok := s.store.State.NoticePreferences[p.owner()]
		if !ok {
			prefs.InApp = true
		}
		if r.Method == "POST" {
			var next NoticePreferences
			if err := decode(w, r, &next); err != nil {
				fail(w, 400, err)
				return
			}
			s.store.State.NoticePreferences[p.owner()] = next
			if err := s.store.save(); err != nil {
				if ok {
					s.store.State.NoticePreferences[p.owner()] = prefs
				} else {
					delete(s.store.State.NoticePreferences, p.owner())
				}
				fail(w, 500, err)
				return
			}
			prefs = next
		} else if r.Method != "GET" {
			fail(w, 405, "method not allowed")
			return
		}
		jsonResponse(w, 200, prefs)
		return
	}
	if r.Method == "GET" && len(parts) == 1 {
		list := []Notice{}
		for _, n := range s.store.State.Notices {
			if n.User == p.owner() && !n.Read {
				list = append(list, n)
			}
		}
		// Preserve unread callback alerts created before the inbox existed.
		knownCallbacks := map[string]bool{}
		for _, n := range s.store.State.Notices {
			if n.User == p.owner() && n.Kind == "callback" {
				knownCallbacks[n.Object] = true
			}
		}
		for _, run := range s.store.State.Runs {
			if run.Owner == p.owner() && run.CallbackFailed && !run.NotificationRead && !knownCallbacks[run.ID] {
				list = append(list, Notice{ID: run.ID, User: run.Owner, Kind: "callback", Title: "异步结果回调最终失败", Object: run.ID, Created: run.Created})
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
		n, ok := s.store.State.Notices[parts[1]]
		if !ok {
			run, exists := s.store.State.Runs[parts[1]]
			if exists && run.Owner == p.owner() && run.CallbackFailed {
				old := run
				run.NotificationRead = true
				s.store.State.Runs[run.ID] = run
				if err := s.store.save(); err != nil {
					s.store.State.Runs[run.ID] = old
					fail(w, 500, err)
					return
				}
				jsonResponse(w, 200, true)
				return
			}
		}
		if !ok || n.User != p.owner() {
			fail(w, 404, "not found")
			return
		}
		old := n
		n.Read = true
		s.store.State.Notices[n.ID] = n
		if err := s.store.save(); err != nil {
			s.store.State.Notices[n.ID] = old
			fail(w, 500, err)
			return
		}
		jsonResponse(w, 200, true)
		return
	}
	fail(w, 405, "method not allowed")
}
func (s *Server) noticeWorker(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.deliverNotice()
		}
	}
}
func (s *Server) deliverNotice() {
	s.store.Lock()
	var n Notice
	var email string
	for _, candidate := range s.store.State.Notices {
		if candidate.EmailStatus == "pending" && !time.Now().Before(candidate.Next) && (n.ID == "" || candidate.Created.Before(n.Created)) {
			n = candidate
		}
	}
	if n.ID == "" {
		s.store.Unlock()
		return
	}
	u := s.store.State.Users[n.User]
	prefs := s.store.State.NoticePreferences[n.User]
	if !prefs.Email || !u.EmailVerified {
		n.EmailStatus = "skipped"
		s.store.State.Notices[n.ID] = n
		_ = s.store.save()
		s.store.Unlock()
		return
	}
	email = u.Email
	s.store.Unlock()
	sender := s.noticeSender
	if sender == nil {
		sender = func(to, title, body string) error { return sendSMTPMessage(to, title, body) }
	}
	err := sender(email, "ToolDeck 通知", n.Title+"\r\n对象："+n.Object+"\r\n请登录 ToolDeck 查看详情。")
	s.store.Lock()
	defer s.store.Unlock()
	current := s.store.State.Notices[n.ID]
	current.Attempts++
	if err == nil {
		current.EmailStatus = "sent"
	} else if current.Attempts >= 5 {
		current.EmailStatus = "failed"
	} else {
		current.Next = time.Now().Add(time.Duration(current.Attempts*current.Attempts) * time.Minute)
	}
	s.store.State.Notices[n.ID] = current
	_ = s.store.save()
}

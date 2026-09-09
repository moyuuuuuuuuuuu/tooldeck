package platform

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"time"
)

type User struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	Nickname      string    `json:"nickname"`
	EmailVerified bool      `json:"email_verified"`
	Email         string    `json:"email"`
	Bio           string    `json:"bio"`
	Admin         bool      `json:"admin"`
	PasswordHash  string    `json:"password_hash"`
	Created       time.Time `json:"created_at"`
}

func passwordHash(password string) string {
	salt := ID("")
	key, _ := pbkdf2.Key(sha256.New, password, []byte(salt), 600000, 32)
	return salt + ":" + hex.EncodeToString(key)
}
func verifyPassword(encoded, password string) bool {
	if len(password) > 128 {
		return false
	}
	parts := strings.Split(encoded, ":")
	if len(parts) != 2 {
		parts = []string{"invalid-account-salt", strings.Repeat("0", 64)}
	}
	key, _ := pbkdf2.Key(sha256.New, password, []byte(parts[0]), 600000, 32)
	return equal(parts[1], hex.EncodeToString(key))
}
func bootstrapUser(st *Store, password string) error {
	if st.State.Users == nil {
		st.State.Users = map[string]User{}
	}
	if _, ok := st.State.Users["admin"]; !ok {
		st.State.Users["admin"] = User{ID: "admin", Username: "admin", Nickname: "ToolDeck 管理员", Admin: true, PasswordHash: passwordHash(password), Created: time.Now()}
	}
	return st.save()
}
func (s *Server) accountRate(w http.ResponseWriter) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	now := time.Now()
	recent := s.attempts["global"][:0]
	for _, t := range s.attempts["global"] {
		if now.Sub(t) < time.Minute {
			recent = append(recent, t)
		}
	}
	s.attempts["global"] = recent
	if len(recent) >= 20 {
		fail(w, 429, "操作频繁，请一分钟后重试")
		return false
	}
	s.attempts["global"] = append(recent, now)
	return true
}
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !s.accountRate(w) {
		return
	}
	var b struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Username string `json:"username"`
		Password string `json:"password"`
		Nickname string `json:"nickname"`
	}
	if e := decode(w, r, &b); e != nil {
		fail(w, 400, e)
		return
	}
	email, valid := normalizeEmail(b.Email)
	if !valid || !regexp.MustCompile(`^[0-9]{6}$`).MatchString(b.Code) {
		fail(w, 422, "请填写有效邮箱和6位验证码")
		return
	}
	b.Username = email
	b.Nickname = strings.TrimSpace(b.Nickname)
	if len(b.Password) < 12 || len(b.Password) > 128 || len([]rune(b.Nickname)) > 50 {
		fail(w, 422, "密码12–128字节，昵称最多50字")
		return
	}
	if b.Nickname == "" {
		b.Nickname = strings.Split(email, "@")[0]
		if len([]rune(b.Nickname)) > 50 {
			b.Nickname = string([]rune(b.Nickname)[:50])
		}
	}
	u := User{Email: email, EmailVerified: true, ID: ID("user_"), Username: b.Username, Nickname: b.Nickname, PasswordHash: passwordHash(b.Password), Created: time.Now()}
	s.store.Lock()
	defer s.store.Unlock()
	for _, existing := range s.store.State.Users {
		if strings.EqualFold(existing.Email, email) {
			fail(w, 409, "该邮箱已注册")
			return
		}
		if existing.Username == u.Username {
			fail(w, 409, "用户名已存在")
			return
		}
	}
	c := s.store.State.EmailCodes[email]
	if c.Hash == "" || time.Now().After(c.Expires) || c.Attempts >= 5 {
		fail(w, 422, "验证码无效或已过期，请重新获取")
		return
	}
	if !equal(c.Hash, emailCodeHash(email, b.Code)) {
		c.Attempts++
		s.store.State.EmailCodes[email] = c
		if e := s.store.save(); e != nil {
			fail(w, 500, "验证码校验失败")
			return
		}
		fail(w, 422, "验证码错误")
		return
	}
	delete(s.store.State.EmailCodes, email)
	s.store.State.Users[u.ID] = u
	if e := s.store.save(); e != nil {
		delete(s.store.State.Users, u.ID)
		s.store.State.EmailCodes[email] = c
		fail(w, 500, e)
		return
	}
	jsonResponse(w, 201, map[string]string{"username": u.Username})
}
func publicUser(u User) map[string]any {
	roles := []string{"R_USER"}
	buttons := []string{}
	if u.Admin {
		roles = []string{"R_SUPER"}
		buttons = []string{"*"}
	}
	return map[string]any{"id": u.ID, "username": u.Username, "nickname": u.Nickname, "email": u.Email, "email_verified": u.EmailVerified, "bio": u.Bio, "avatar": "", "roles": roles, "buttons": buttons, "created_at": u.Created}
}
func (s *Server) profile(w http.ResponseWriter, r *http.Request, p Principal) {
	if r.Method != "GET" && r.Method != "PATCH" {
		fail(w, 405, "GET or PATCH required")
		return
	}
	var b struct {
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Bio      string `json:"bio"`
	}
	if r.Method == "PATCH" {
		if e := decode(w, r, &b); e != nil {
			fail(w, 400, e)
			return
		}
		b.Nickname = strings.TrimSpace(b.Nickname)
		b.Email = strings.TrimSpace(b.Email)
		if b.Nickname == "" || len([]rune(b.Nickname)) > 50 || len(b.Email) > 254 || len([]rune(b.Bio)) > 500 {
			fail(w, 422, "昵称1–50字，邮箱最多254字节，简介最多500字")
			return
		}
		if b.Email != "" {
			a, e := mail.ParseAddress(b.Email)
			if e != nil || a.Address != b.Email {
				fail(w, 422, "邮箱格式错误")
				return
			}
		}
	}
	s.store.Lock()
	defer s.store.Unlock()
	u := s.store.State.Users[p.owner()]
	if r.Method == "PATCH" {
		if b.Email != u.Email {
			fail(w, 422, "注册邮箱暂不支持修改")
			return
		}
		old := u
		u.Nickname = b.Nickname
		u.Email = b.Email
		u.Bio = b.Bio
		s.store.State.Users[u.ID] = u
		if e := s.store.save(); e != nil {
			s.store.State.Users[u.ID] = old
			fail(w, 500, e)
			return
		}
	}
	jsonResponse(w, 200, publicUser(u))
}
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, p Principal) {
	if r.Method != "POST" {
		fail(w, 405, "POST required")
		return
	}
	if !s.accountRate(w) {
		return
	}
	var b struct {
		Current  string `json:"current_password"`
		Password string `json:"password"`
	}
	if e := decode(w, r, &b); e != nil {
		fail(w, 400, e)
		return
	}
	if len(b.Password) < 12 || len(b.Password) > 128 {
		fail(w, 422, "新密码需12–128字节")
		return
	}
	s.store.Lock()
	u := s.store.State.Users[p.owner()]
	s.store.Unlock()
	if !verifyPassword(u.PasswordHash, b.Current) {
		fail(w, 422, "当前密码错误")
		return
	}
	encoded := passwordHash(b.Password)
	s.store.Lock()
	defer s.store.Unlock()
	current := s.store.State.Users[u.ID]
	if current.PasswordHash != u.PasswordHash {
		fail(w, 409, "密码已变更，请重新登录")
		return
	}
	current.PasswordHash = encoded
	s.store.State.Users[u.ID] = current
	removed := map[string]Credential{}
	for id, k := range s.store.State.Keys {
		if k.Session && (k.UserID == u.ID || k.UserID == "" && u.ID == "admin") {
			removed[id] = k
			delete(s.store.State.Keys, id)
		}
	}
	if e := s.store.save(); e != nil {
		current.PasswordHash = u.PasswordHash
		s.store.State.Users[u.ID] = current
		for id, k := range removed {
			s.store.State.Keys[id] = k
		}
		fail(w, 500, e)
		return
	}
	jsonResponse(w, 200, true)
}
func (s *Server) statistics(w http.ResponseWriter, r *http.Request, p Principal) {
	if r.Method != "GET" {
		fail(w, 405, "GET required")
		return
	}
	type Counter struct {
		ToolID    string     `json:"tool_id"`
		Title     string     `json:"title"`
		Version   string     `json:"version"`
		Total     int        `json:"total"`
		Succeeded int        `json:"succeeded"`
		Failed    int        `json:"failed"`
		Pending   int        `json:"pending"`
		Canceled  int        `json:"canceled"`
		Last      *time.Time `json:"last_used_at"`
	}
	s.store.Lock()
	defer s.store.Unlock()
	all := Counter{}
	week := 0
	byTool := map[string]*Counter{}
	for _, run := range s.store.State.Runs {
		if run.Owner != p.owner() {
			continue
		}
		c := byTool[run.ToolID]
		if c == nil {
			t := s.store.State.Tools[run.ToolID]
			c = &Counter{ToolID: run.ToolID, Title: t.Manifest.Title, Version: t.Manifest.Version}
			byTool[run.ToolID] = c
		}
		for _, v := range []*Counter{&all, c} {
			v.Total++
			switch run.Status {
			case "succeeded":
				v.Succeeded++
			case "failed", "timed_out", "cancel_failed":
				v.Failed++
			case "queued", "running", "canceling":
				v.Pending++
			case "canceled":
				v.Canceled++
			}
			if v.Last == nil || run.Created.After(*v.Last) {
				t := run.Created
				v.Last = &t
			}
		}
		if run.Created.After(time.Now().Add(-7 * 24 * time.Hour)) {
			week++
		}
	}
	list := []*Counter{}
	for _, c := range byTool {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Total > list[j].Total })
	jsonResponse(w, 200, map[string]any{"summary": all, "last_seven_days": week, "tools": list})
}

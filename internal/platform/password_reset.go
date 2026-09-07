package platform

import (
	"net/http"
	"regexp"
	"strings"
	"time"
)

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	if !s.accountRate(w) {
		return
	}
	var b struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if e := decode(w, r, &b); e != nil {
		fail(w, 400, e)
		return
	}
	email, ok := normalizeEmail(b.Email)
	if !ok || !regexp.MustCompile(`^[0-9]{6}$`).MatchString(b.Code) {
		fail(w, 422, "请填写有效邮箱和6位验证码")
		return
	}
	if len(b.Password) < 12 || len(b.Password) > 128 {
		fail(w, 422, "新密码需12–128字节")
		return
	}
	key := "reset:" + email
	s.store.Lock()
	defer s.store.Unlock()
	c := s.store.State.EmailCodes[key]
	u, exists := s.store.State.Users[c.UserID]
	if !exists || !u.EmailVerified || !strings.EqualFold(u.Email, email) || c.Hash == "" || !time.Now().Before(c.Expires) || c.Attempts >= 5 {
		fail(w, 422, "验证码无效或已过期，请重新获取")
		return
	}
	if !equal(c.Hash, emailCodeHash(key, b.Code)) {
		c.Attempts++
		s.store.State.EmailCodes[key] = c
		if e := s.store.save(); e != nil {
			fail(w, 500, "验证码校验失败")
			return
		}
		fail(w, 422, "验证码错误")
		return
	}
	oldUser := u
	u.PasswordHash = passwordHash(b.Password)
	s.store.State.Users[u.ID] = u
	consumed := c
	consumed.Hash = "" // Retain cooldown and daily quota after consumption.
	s.store.State.EmailCodes[key] = consumed
	removed := map[string]Credential{}
	for id, k := range s.store.State.Keys {
		if k.Session && (k.UserID == u.ID || k.UserID == "" && u.ID == "admin") {
			removed[id] = k
			delete(s.store.State.Keys, id)
		}
	}
	if e := s.store.save(); e != nil {
		s.store.State.Users[u.ID] = oldUser
		s.store.State.EmailCodes[key] = c
		for id, k := range removed {
			s.store.State.Keys[id] = k
		}
		fail(w, 500, "密码保存失败，请重试")
		return
	}
	jsonResponse(w, 200, true)
}

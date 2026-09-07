package platform

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPasswordReset(t *testing.T) {
	s := testServer(t)
	email := "alice@example.com"
	key := "reset:" + email
	old := passwordHash("old-password-123")
	s.store.State.Users["alice"] = User{ID: "alice", Email: email, EmailVerified: true, PasswordHash: old}
	s.store.State.Keys["session"] = Credential{Session: true, UserID: "alice"}
	s.store.State.Keys["other"] = Credential{Session: true, UserID: "bob"}
	s.store.State.Keys["api"] = Credential{UserID: "alice"}
	sent := ""
	deliveryFails := false
	s.mailSender = func(to, code string) error {
		sent = code
		if deliveryFails {
			return errors.New("failed")
		}
		return nil
	}
	call := func(path, body string, want int) {
		t.Helper()
		s.attempts["global"] = nil
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/core/password-reset"+path, strings.NewReader(body)))
		if w.Code != want {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if sent != "" && strings.Contains(w.Body.String(), sent) {
			t.Fatal("code leaked")
		}
	}
	body := func(code string) string {
		return fmt.Sprintf(`{"email":%q,"code":%q,"password":"new-password-123"}`, email, code)
	}
	call("/email-code", `{"email":"missing@example.com"}`, 200)
	if sent != "" {
		t.Fatal("mail sent to unknown account")
	}
	call("/email-code", `{"email":"ALICE@example.com"}`, 200)
	call("/email-code", `{"email":"alice@example.com"}`, 429)
	c := s.store.State.EmailCodes[key]
	s.store.State.EmailCodes[key] = EmailCode{}
	s.store.State.EmailCodes[email] = EmailCode{Hash: emailCodeHash(email, sent), Expires: time.Now().Add(time.Minute)}
	call("", body(sent), 422) // Registration code must not reset a password.
	s.store.State.EmailCodes[key] = c
	expired := c
	expired.Expires = time.Now().Add(-time.Second)
	s.store.State.EmailCodes[key] = expired
	call("", body(sent), 422)
	s.store.State.EmailCodes[key] = c
	wrong := "000000"
	if sent == wrong {
		wrong = "111111"
	}
	for i := 0; i < 5; i++ {
		call("", body(wrong), 422)
	}
	call("", body(sent), 422)
	c.Sent = time.Now().Add(-time.Minute)
	s.store.State.EmailCodes[key] = c
	deliveryFails = true
	call("/email-code", `{"email":"alice@example.com"}`, 503)
	call("", body(sent), 422)
	c = s.store.State.EmailCodes[key]
	c.Sent = time.Now().Add(-time.Minute)
	s.store.State.EmailCodes[key] = c
	deliveryFails = false
	call("/email-code", `{"email":"alice@example.com"}`, 200)
	call("", body(sent), 200)
	if !verifyPassword(s.store.State.Users["alice"].PasswordHash, "new-password-123") || verifyPassword(s.store.State.Users["alice"].PasswordHash, "old-password-123") {
		t.Fatal("password not replaced")
	}
	if _, ok := s.store.State.Keys["session"]; ok {
		t.Fatal("session retained")
	}
	if len(s.store.State.Keys) != 2 {
		t.Fatal("unrelated credentials changed")
	}
	call("", body(sent), 422)
	call("/email-code", `{"email":"alice@example.com"}`, 429)
}

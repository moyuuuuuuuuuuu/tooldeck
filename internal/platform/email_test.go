package platform

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRegistrationEmailVerification(t *testing.T) {
	s := testServer(t)
	var sent string
	s.mailSender = func(email, code string) error { sent = code; return nil }
	call := func(path, body string, want int) {
		t.Helper()
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(body)))
		if w.Code != want {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if sent != "" && strings.Contains(w.Body.String(), sent) {
			t.Fatal("code leaked in response")
		}
	}
	call("/api/core/register", `{"username":"newuser","password":"password-123456"}`, 422)
	call("/api/core/register/email-code", `{"email":"not-an-email"}`, 422)
	call("/api/core/register/email-code", `{"email":"new@example.com"}`, 200)
	if len(sent) != 6 {
		t.Fatal("expected six-digit email code")
	}
	call("/api/core/register/email-code", `{"email":"new@example.com"}`, 429)
	body := func(email, code string) string {
		return fmt.Sprintf(`{"username":"newuser","password":"password-123456","email":%q,"code":%q}`, email, code)
	}
	call("/api/core/register", body("other@example.com", sent), 422)
	wrong := "000000"
	if sent == wrong {
		wrong = "111111"
	}
	for i := 0; i < 5; i++ {
		call("/api/core/register", body("new@example.com", wrong), 422)
	}
	call("/api/core/register", body("new@example.com", sent), 422)
	s.store.Lock()
	c := s.store.State.EmailCodes["new@example.com"]
	c.Sent = time.Now().Add(-time.Minute)
	s.store.State.EmailCodes["new@example.com"] = c
	s.store.Unlock()
	call("/api/core/register/email-code", `{"email":"new@example.com"}`, 200)
	s.store.Lock()
	c = s.store.State.EmailCodes["new@example.com"]
	c.Expires = time.Now().Add(-time.Second)
	s.store.State.EmailCodes["new@example.com"] = c
	s.store.Unlock()
	call("/api/core/register", body("new@example.com", sent), 422)
	s.store.Lock()
	c.Expires = time.Now().Add(time.Minute)
	s.store.State.EmailCodes["new@example.com"] = c
	s.store.Unlock()
	call("/api/core/register", body("new@example.com", sent), 201)
	s.store.Lock()
	defer s.store.Unlock()
	if _, ok := s.store.State.EmailCodes["new@example.com"]; ok {
		t.Fatal("code not consumed")
	}
	for _, u := range s.store.State.Users {
		if u.Username == "new@example.com" && !u.EmailVerified {
			t.Fatal("email not verified")
		}
	}
}
func TestFailedMailCannotRegister(t *testing.T) {
	s := testServer(t)
	var sent string
	s.mailSender = func(email, code string) error { sent = code; return errors.New("delivery failed") }
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/core/register/email-code", strings.NewReader(`{"email":"new@example.com"}`)))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/core/register", strings.NewReader(fmt.Sprintf(`{"username":"newuser","password":"password-123456","email":"new@example.com","code":%q}`, sent))))
	if w.Code != 422 {
		t.Fatal(w.Code)
	}
}

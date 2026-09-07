package platform

import (
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type EmailCode struct {
	UserID   string    `json:"user_id,omitempty"`
	Hash     string    `json:"hash"`
	Sent     time.Time `json:"sent"`
	Expires  time.Time `json:"expires"`
	Attempts int       `json:"attempts"`
	Window   time.Time `json:"window"`
	Count    int       `json:"count"`
}

func normalizeEmail(v string) (string, bool) {
	v = strings.ToLower(strings.TrimSpace(v))
	a, e := mail.ParseAddress(v)
	return v, e == nil && a.Address == v && len(v) <= 254 && !strings.ContainsAny(v, "\r\n")
}
func emailCodeHash(email, code string) string {
	return hash(os.Getenv("TOOLDECK_MASTER_KEY") + ":" + email + ":" + code)
}
func (s *Server) sendEmailCode(w http.ResponseWriter, r *http.Request) {
	s.sendAccountCode(w, r, false)
}
func (s *Server) sendAccountCode(w http.ResponseWriter, r *http.Request, reset bool) {
	if !s.accountRate(w) {
		return
	}
	var b struct {
		Email string `json:"email"`
	}
	if e := decode(w, r, &b); e != nil {
		fail(w, 400, e)
		return
	}
	email, ok := normalizeEmail(b.Email)
	if !ok {
		fail(w, 422, "请填写有效邮箱")
		return
	}
	if s.mailSender == nil && (os.Getenv("TOOLDECK_SMTP_HOST") == "" || os.Getenv("TOOLDECK_SMTP_FROM") == "") {
		fail(w, 503, "邮件服务尚未配置，请稍后再试")
		return
	}
	n, e := rand.Int(rand.Reader, big.NewInt(1000000))
	if e != nil {
		fail(w, 500, "验证码生成失败")
		return
	}
	code := fmt.Sprintf("%06d", n)
	now := time.Now()
	s.store.Lock()
	key := email
	userID := ""
	for _, u := range s.store.State.Users {
		if strings.EqualFold(u.Email, email) {
			if !reset {
				s.store.Unlock()
				fail(w, 409, "该邮箱已注册")
				return
			}
			if u.EmailVerified {
				userID = u.ID
			}
		}
	}
	if reset {
		key = "reset:" + email
		if userID == "" {
			s.store.Unlock()
			jsonResponse(w, 200, map[string]int{"retry_after": 60, "expires_in": 600})
			return
		}
	}
	if s.store.State.EmailCodes == nil {
		s.store.State.EmailCodes = map[string]EmailCode{}
	}
	c := s.store.State.EmailCodes[key]
	if now.Sub(c.Sent) < time.Minute {
		s.store.Unlock()
		fail(w, 429, "请60秒后重新发送")
		return
	}
	if now.Sub(c.Window) >= 24*time.Hour {
		c.Count = 0
		c.Window = now
	}
	if c.Count >= 10 {
		s.store.Unlock()
		fail(w, 429, "该邮箱今日发送次数已达上限")
		return
	}
	// Empty hash reserves the cooldown but cannot be used to register until delivery succeeds.
	c.UserID = userID
	c.Sent = now
	c.Expires = now.Add(10 * time.Minute)
	c.Attempts = 0
	c.Count++
	c.Hash = ""
	for address, old := range s.store.State.EmailCodes {
		if now.Sub(old.Window) >= 24*time.Hour {
			delete(s.store.State.EmailCodes, address)
		}
	}
	s.store.State.EmailCodes[key] = c
	if e = s.store.save(); e != nil {
		s.store.Unlock()
		fail(w, 500, "验证码保存失败")
		return
	}
	s.store.Unlock()
	sender := s.mailSender
	if sender == nil {
		sender = sendSMTPCode
		if reset {
			sender = func(to, code string) error {
				return sendSMTPMessage(to, "ToolDeck password reset", "你的 ToolDeck 密码重置验证码是："+code+"\r\n验证码10分钟内有效，请勿向他人透露。若非本人操作，请忽略此邮件。")
			}
		}
	}
	e = sender(email, code)
	s.store.Lock()
	defer s.store.Unlock()
	if e != nil {
		fail(w, 503, "验证码发送失败，请稍后重试")
		return
	}
	if !s.store.State.EmailCodes[key].Sent.Equal(c.Sent) {
		fail(w, 409, "验证码已更新，请使用最新邮件")
		return
	}
	c.Hash = emailCodeHash(key, code)
	s.store.State.EmailCodes[key] = c
	if e = s.store.save(); e != nil {
		c.Hash = ""
		s.store.State.EmailCodes[key] = c
		fail(w, 500, "验证码保存失败，请稍后重试")
		return
	}
	jsonResponse(w, 200, map[string]int{"retry_after": 60, "expires_in": 600})
}
func sendSMTPCode(to, code string) error {
	return sendSMTPMessage(to, "ToolDeck email verification", "你的 ToolDeck 注册验证码是："+code+"\r\n验证码10分钟内有效，请勿向他人透露。若非本人操作，请忽略此邮件。")
}
func sendSMTPMessage(to, subject, body string) error {
	host, port := os.Getenv("TOOLDECK_SMTP_HOST"), os.Getenv("TOOLDECK_SMTP_PORT")
	if port == "" {
		port = "465"
	}
	from, ok := normalizeEmail(os.Getenv("TOOLDECK_SMTP_FROM"))
	if !ok {
		return fmt.Errorf("invalid SMTP sender")
	}
	mode := os.Getenv("TOOLDECK_SMTP_SECURITY")
	if mode == "" {
		mode = "tls"
	}
	if mode != "tls" && mode != "starttls" {
		return fmt.Errorf("SMTP requires tls or starttls")
	}
	conn, e := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
	if e != nil {
		return e
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	config := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if mode == "tls" {
		tc := tls.Client(conn, config)
		if e = tc.Handshake(); e != nil {
			return e
		}
		conn = tc
	}
	client, e := smtp.NewClient(conn, host)
	if e != nil {
		return e
	}
	defer client.Close()
	if mode == "starttls" {
		if e = client.StartTLS(config); e != nil {
			return e
		}
	}
	if user := os.Getenv("TOOLDECK_SMTP_USERNAME"); user != "" {
		if e = client.Auth(smtp.PlainAuth("", user, os.Getenv("TOOLDECK_SMTP_PASSWORD"), host)); e != nil {
			return e
		}
	}
	if e = client.Mail(from); e != nil {
		return e
	}
	if e = client.Rcpt(to); e != nil {
		return e
	}
	writer, e := client.Data()
	if e != nil {
		return e
	}
	_, e = fmt.Fprintf(writer, "From: ToolDeck <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", from, to, subject, body)
	if e != nil {
		return e
	}
	if e = writer.Close(); e != nil {
		return e
	}
	return client.Quit()
}

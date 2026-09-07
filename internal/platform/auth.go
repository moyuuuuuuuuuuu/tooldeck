package platform

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Principal struct {
	UserID  string
	Session bool
	ID      string
	Admin   bool
	Tools   []string
}

func (p Principal) allows(name string) bool {
	if p.Admin {
		return true
	}
	for _, t := range p.Tools {
		if t == name || t == "*" {
			return true
		}
	}
	return false
}
func hash(v string) string { b := sha256.Sum256([]byte(v)); return hex.EncodeToString(b[:]) }
func equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(hash(a)), []byte(hash(b))) == 1
}
func (s *Server) authenticate(r *http.Request) (Principal, error) {
	token := r.Header.Get("X-API-Key")
	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = bearer
	}
	if token == "" {
		return Principal{}, errors.New("authentication required")
	}
	s.store.Lock()
	for _, k := range s.store.State.Keys {
		if equal(k.Hash, hash(token)) && (time.Now().Before(k.Expires) || (k.NeverExpires && !k.Session)) {
			uid := k.UserID
			if uid == "" && k.Session {
				uid = "admin"
			}
			p := Principal{ID: k.ID, UserID: uid, Session: k.Session, Tools: k.Tools}
			if uid != "" {
				u, ok := s.store.State.Users[uid]
				if !ok {
					s.store.Unlock()
					return Principal{}, errors.New("invalid user")
				}
				p.Admin = k.Session && u.Admin
				if k.Session {
					p.Tools = []string{"*"}
				}
			}
			s.store.Unlock()
			return p, nil
		}
	}
	s.store.Unlock()
	if token != bearer || strings.HasPrefix(token, "td_") || s.oauthURL == "" {
		return Principal{}, errors.New("invalid or expired credential")
	}
	// OAuth resource server: accept only active introspection results with the configured audience and execution scope.
	form := url.Values{"token": {token}}
	req, e := http.NewRequestWithContext(r.Context(), "POST", s.oauthURL, strings.NewReader(form.Encode()))
	if e != nil {
		return Principal{}, e
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(os.Getenv("TOOLDECK_OAUTH_CLIENT_ID"), os.Getenv("TOOLDECK_OAUTH_CLIENT_SECRET"))
	resp, e := (&http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if e != nil {
		return Principal{}, errors.New("OAuth verification unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Principal{}, errors.New("OAuth verification failed")
	}
	var v struct {
		Active bool   `json:"active"`
		Sub    string `json:"sub"`
		Scope  string `json:"scope"`
		Aud    any    `json:"aud"`
		Exp    int64  `json:"exp"`
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&v); e != nil {
		return Principal{}, e
	}
	aud, _ := json.Marshal(v.Aud)
	var audiences []string
	if x, ok := v.Aud.(string); ok {
		audiences = []string{x}
	} else {
		_ = json.Unmarshal(aud, &audiences)
	}
	ok := false
	for _, a := range audiences {
		if a == os.Getenv("TOOLDECK_OAUTH_AUDIENCE") && a != "" {
			ok = true
		}
	}
	if !v.Active || v.Sub == "" || v.Exp <= time.Now().Unix() || !ok {
		return Principal{}, errors.New("invalid OAuth token")
	}
	var tools []string
	for _, x := range strings.Fields(v.Scope) {
		if strings.HasPrefix(x, "tool:") {
			tools = append(tools, strings.TrimPrefix(x, "tool:"))
		}
	}
	if len(tools) == 0 {
		return Principal{}, errors.New("OAuth token has no tool scopes")
	}
	return Principal{ID: "oauth:" + v.Sub, Tools: tools}, nil
}
func (s *Server) crypt() (cipher.AEAD, error) {
	k, e := hex.DecodeString(os.Getenv("TOOLDECK_MASTER_KEY"))
	if e != nil || len(k) != 32 {
		return nil, errors.New("TOOLDECK_MASTER_KEY must be 64 hexadecimal characters")
	}
	b, e := aes.NewCipher(k)
	if e != nil {
		return nil, e
	}
	return cipher.NewGCM(b)
}
func (s *Server) encrypt(v string) (string, error) {
	a, e := s.crypt()
	if e != nil {
		return "", e
	}
	nonce, _ := hex.DecodeString(ID("")[:a.NonceSize()*2])
	return base64.StdEncoding.EncodeToString(a.Seal(nonce, nonce, []byte(v), nil)), nil
}
func (s *Server) decrypt(v string) (string, error) {
	a, e := s.crypt()
	if e != nil {
		return "", e
	}
	b, e := base64.StdEncoding.DecodeString(v)
	if e != nil || len(b) < a.NonceSize() {
		return "", errors.New("invalid secret")
	}
	p, e := a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], nil)
	return string(p), e
}

func (p Principal) owner() string {
	if p.UserID != "" {
		return p.UserID
	}
	if p.Admin {
		return "admin"
	}
	return p.ID
}

package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var callbackRetryDelays = [...]time.Duration{3 * time.Second, 30 * time.Second, 300 * time.Second, 3000 * time.Second, 300000 * time.Second}

type callbackArtifact struct {
	FileID      string `json:"file_id"`
	Name        string `json:"name"`
	MIME        string `json:"mime"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url,omitempty"`
}

type callbackPayload struct {
	Event       string             `json:"event"`
	RunID       string             `json:"run_id"`
	ToolID      string             `json:"tool_id"`
	Status      string             `json:"status"`
	Result      any                `json:"result"`
	Error       string             `json:"error,omitempty"`
	CancelError string             `json:"cancel_error,omitempty"`
	Artifacts   []callbackArtifact `json:"artifacts"`
	CreatedAt   time.Time          `json:"created_at"`
	StartedAt   *time.Time         `json:"started_at,omitempty"`
	DurationMS  int64              `json:"duration_ms"`
}

func validateCallbackURL(raw string) error {
	if len(raw) == 0 || len(raw) > 2048 {
		return errors.New("异步 API 调用必须提供有效的 callback_url")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("callback_url 必须是无账号信息和片段的 HTTPS 地址")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return errors.New("callback_url 端口无效")
		}
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !publicCallbackIP(ip) {
		return errors.New("callback_url 不能指向私有或保留地址")
	}
	return nil
}

func publicCallbackIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsUnspecified() && !ip.IsMulticast() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
}

func callbackHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(ips) == 0 {
				return nil, errors.New("callback host cannot be resolved")
			}
			for _, resolved := range ips {
				if !publicCallbackIP(resolved.IP) {
					return nil, errors.New("callback host resolves to a private or reserved address")
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
	}
	return &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func sendCallback(rawURL, secret string, payload callbackPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ToolDeck-Callback/1.0")
	req.Header.Set("X-ToolDeck-Event", payload.Event)
	req.Header.Set("X-ToolDeck-Run-ID", payload.RunID)
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		req.Header.Set("X-ToolDeck-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := callbackHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("callback returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func terminalRun(status string) bool {
	return status != "" && !runActive(status)
}

func (s *Server) callbackWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.deliverNextCallback()
		}
	}
}

func (s *Server) deliverNextCallback() {
	now := time.Now()
	s.store.Lock()
	var selected Run
	for _, run := range s.store.State.Runs {
		if run.CallbackURL != "" && terminalRun(run.Status) && !run.CallbackSent && !run.CallbackFailed && !now.Before(run.CallbackNext) {
			selected = run
			break
		}
	}
	if selected.ID == "" {
		s.store.Unlock()
		return
	}
	cipher := s.store.State.CallbackSecrets[selected.ID]
	secret := ""
	if cipher != "" {
		var err error
		secret, err = s.decrypt(cipher)
		if err != nil {
			selected.CallbackAttempts = len(callbackRetryDelays) + 1
			selected.CallbackFailed = true
			selected.CallbackError = "回调密钥解密失败"
			s.store.State.Runs[selected.ID] = selected
			delete(s.store.State.CallbackSecrets, selected.ID)
			_ = s.store.save()
			s.store.Unlock()
			return
		}
	}
	payload := callbackPayload{Event: "tool.run.completed", RunID: selected.ID, ToolID: selected.ToolID, Status: selected.Status, Result: selected.Result, Error: selected.Error, CancelError: selected.CancelError, Artifacts: []callbackArtifact{}, CreatedAt: selected.Created, StartedAt: selected.Started, DurationMS: selected.Duration}
	for _, artifact := range selected.Artifacts {
		payload.Artifacts = append(payload.Artifacts, callbackArtifact{FileID: artifact.ID, Name: artifact.Name, MIME: artifact.MIME, Size: artifact.Size, DownloadURL: s.artifactURL(artifact)})
	}
	s.store.Unlock()
	sender := s.callbackSender
	if sender == nil {
		sender = sendCallback
	}
	err := sender(selected.CallbackURL, secret, payload)
	s.store.Lock()
	defer s.store.Unlock()
	current, ok := s.store.State.Runs[selected.ID]
	if !ok || current.CallbackSent || current.CallbackFailed || current.CallbackAttempts != selected.CallbackAttempts {
		return
	}
	current.CallbackAttempts++
	if err == nil {
		current.CallbackSent = true
		current.CallbackError = ""
		delete(s.store.State.CallbackSecrets, current.ID)
	} else {
		current.CallbackError = err.Error()
		if len([]rune(current.CallbackError)) > 500 {
			current.CallbackError = string([]rune(current.CallbackError)[:500])
		}
		if current.CallbackAttempts > len(callbackRetryDelays) {
			current.CallbackFailed = true
			delete(s.store.State.CallbackSecrets, current.ID)
		} else {
			current.CallbackNext = time.Now().Add(callbackRetryDelays[current.CallbackAttempts-1])
		}
	}
	s.store.State.Runs[current.ID] = current
	if err := s.store.save(); err != nil {
		fmt.Fprintln(os.Stderr, "callback state save failed:", err)
	}
}

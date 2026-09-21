package platform

import (
	"encoding/base64"
	"net/http"
	"strings"
)

type DonationSettings struct {
	Message  string `json:"message"`
	WeChatQR string `json:"wechat_qr"`
	AlipayQR string `json:"alipay_qr"`
}

func donationImage(value string) bool {
	if value == "" {
		return true
	}
	for _, mime := range []string{"image/png", "image/jpeg", "image/webp"} {
		prefix := "data:" + mime + ";base64,"
		if strings.HasPrefix(value, prefix) {
			b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
			return err == nil && len(b) <= 512<<10 && http.DetectContentType(b) == mime
		}
	}
	return false
}

func (s *Server) donationEndpoint(w http.ResponseWriter, r *http.Request, p Principal, public bool) {
	if !public && (!p.Admin || !p.Session) {
		fail(w, 403, "admin session required")
		return
	}
	if r.Method != "GET" && (public || r.Method != "POST") {
		fail(w, 405, "method not allowed")
		return
	}
	var settings DonationSettings
	if r.Method == "POST" {
		if err := decode(w, r, &settings); err != nil {
			fail(w, 400, err)
			return
		}
		if len(settings.Message) > 4000 || !donationImage(settings.WeChatQR) || !donationImage(settings.AlipayQR) {
			fail(w, 422, "说明最多 4000 字节；收款码支持不超过 512 KB 的 PNG/JPEG/WebP")
			return
		}
	}
	s.store.Lock()
	defer s.store.Unlock()
	if r.Method == "POST" {
		previous := s.store.State.Donation
		s.store.State.Donation = settings
		if err := s.store.save(); err != nil {
			s.store.State.Donation = previous
			fail(w, 500, err)
			return
		}
	}
	jsonResponse(w, 200, s.store.State.Donation)
}

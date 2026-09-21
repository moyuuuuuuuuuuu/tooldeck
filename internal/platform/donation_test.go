package platform

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDonationSettings(t *testing.T) {
	s := testServer(t)
	admin := Principal{Admin: true, Session: true}
	call := func(method, body string, p Principal, public bool, want int) string {
		t.Helper()
		w := httptest.NewRecorder()
		s.donationEndpoint(w, httptest.NewRequest(method, "/", strings.NewReader(body)), p, public)
		if w.Code != want {
			t.Fatalf("got %d, want %d: %s", w.Code, want, w.Body.String())
		}
		return w.Body.String()
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	qr := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	settings := DonationSettings{Message: "感谢支持", WeChatQR: qr, AlipayQR: qr}
	body, _ := json.Marshal(settings)
	call("POST", string(body), Principal{}, false, 403)
	call("POST", string(body), Principal{Admin: true}, false, 403)
	call("POST", string(body), admin, true, 405)
	call("POST", string(body), admin, false, 200)
	if !strings.Contains(call("GET", "", Principal{}, true, 200), qr) {
		t.Fatal("public QR missing")
	}
	call("POST", `{"wechat_qr":"data:image/png;base64,PHNjcmlwdD4="}`, admin, false, 422)
	call("POST", `{"alipay_qr":"javascript:alert(1)"}`, admin, false, 422)
	buf.Reset()
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 3))); err != nil {
		t.Fatal(err)
	}
	settings.WeChatQR = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	body, _ = json.Marshal(settings)
	call("POST", string(body), admin, false, 200)
	if s.store.State.Donation.WeChatQR == qr || s.store.State.Donation.AlipayQR != qr {
		t.Fatal("replacement must update only the selected channel")
	}
	settings.WeChatQR = ""
	settings.Message = "已更新"
	body, _ = json.Marshal(settings)
	call("POST", string(body), admin, false, 200)
	st, err := OpenStore(s.store.Root)
	if err != nil {
		t.Fatal(err)
	}
	if st.State.Donation != settings {
		t.Fatal("replacement not persisted")
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/public/donation", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "已更新") {
		t.Fatal("public route unavailable", w.Body.String())
	}
}

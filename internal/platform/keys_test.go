package platform

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestPermanentKeyAuthentication(t *testing.T) {
	s := testServer(t)
	r := httptest.NewRequest("GET", "/api/v1/tools", nil)
	r.Header.Set("X-API-Key", "td_key_test")
	k := Credential{ID: "key_test", Hash: hash("td_key_test"), Tools: []string{"*"}, NeverExpires: true}
	s.store.State.Keys[k.ID] = k
	p, err := s.authenticate(r)
	if err != nil || !p.allows("future-tool") {
		t.Fatal("permanent wildcard key rejected", err)
	}
	k.NeverExpires = false
	k.Expires = time.Now().Add(-time.Hour)
	s.store.State.Keys[k.ID] = k
	if _, err = s.authenticate(r); err == nil {
		t.Fatal("expired key accepted")
	}
	k.NeverExpires = true
	k.Session = true
	s.store.State.Keys[k.ID] = k
	if _, err = s.authenticate(r); err == nil {
		t.Fatal("expired session accepted")
	}
	delete(s.store.State.Keys, k.ID)
	if _, err = s.authenticate(r); err == nil {
		t.Fatal("revoked key accepted")
	}
}

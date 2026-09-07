package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func publicIP(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !net.ParseIP("100.64.0.0").Equal(ip) && !inCIDR(ip, "100.64.0.0/10") && !inCIDR(ip, "198.18.0.0/15") && !inCIDR(ip, "0.0.0.0/8")
}
func inCIDR(ip net.IP, c string) bool { _, n, _ := net.ParseCIDR(c); return n.Contains(ip) }
func egressProxy(hosts []string) http.Handler {
	allowed := map[string]bool{}
	for _, h := range hosts {
		allowed[strings.ToLower(h)] = true
	}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		h, p, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		if !allowed[strings.ToLower(h)] || p != "443" && p != "80" {
			return nil, errors.New("destination not allowed")
		}
		ips, e := lookupEgress(ctx, h)
		if e != nil {
			return nil, e
		}
		if len(ips) == 0 {
			return nil, errors.New("no destination")
		}
		for _, ip := range ips {
			if !publicIP(ip.IP) {
				return nil, errors.New("private destinations are blocked")
			}
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ips[0].IP.String(), p))
	}
	transport := &http.Transport{DialContext: dial, ResponseHeaderTimeout: 30 * time.Second, DisableKeepAlives: true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "CONNECT" {
			conn, e := dial(r.Context(), "tcp", r.Host)
			if e != nil {
				http.Error(w, "destination denied", 403)
				return
			}
			h, ok := w.(http.Hijacker)
			if !ok {
				conn.Close()
				http.Error(w, "unsupported", 500)
				return
			}
			client, rw, e := h.Hijack()
			if e != nil {
				conn.Close()
				return
			}
			defer conn.Close()
			defer client.Close()
			deadline := time.Now().Add(15 * time.Minute)
			conn.SetDeadline(deadline)
			client.SetDeadline(deadline)
			rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
			rw.Flush()
			go io.Copy(conn, rw)
			io.Copy(client, conn)
			return
		}
		if r.URL.Scheme != "http" {
			http.Error(w, "HTTP or HTTPS proxy required", 400)
			return
		}
		r.RequestURI = ""
		r.Header.Del("Proxy-Authorization")
		r.Header.Del("Proxy-Connection")
		resp, e := transport.RoundTrip(r)
		if e != nil {
			http.Error(w, "destination denied or unavailable", 502)
			return
		}
		defer resp.Body.Close()
		for k, v := range resp.Header {
			for _, x := range v {
				w.Header().Add(k, x)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})
}

func lookupEgress(ctx context.Context, host string) ([]net.IPAddr, error) {
	endpoint := os.Getenv("TOOLDECK_EGRESS_DOH")
	if endpoint == "" {
		return net.DefaultResolver.LookupIPAddr(ctx, host)
	}
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" {
		return nil, errors.New("DoH endpoint must use HTTPS")
	}
	q := u.Query()
	q.Set("name", host)
	q.Set("type", "A")
	q.Set("edns_client_subnet", "0.0.0.0/0")
	u.RawQuery = q.Encode()
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/dns-json")
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, errors.New("DoH request failed")
	}
	var answer struct {
		Status int
		Answer []struct {
			Type int
			Data string
		}
	}
	if e = json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&answer); e != nil {
		return nil, e
	}
	if answer.Status != 0 {
		return nil, errors.New("DNS resolution failed")
	}
	var ips []net.IPAddr
	for _, a := range answer.Answer {
		if a.Type == 1 {
			ip := net.ParseIP(a.Data)
			if ip == nil {
				return nil, errors.New("invalid DNS answer")
			}
			ips = append(ips, net.IPAddr{IP: ip})
		}
	}
	return ips, nil
}

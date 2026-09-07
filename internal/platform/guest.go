package platform

import (
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

func guestTool(t Tool) bool {
	return !t.Playground && !t.Withdrawn && (t.Public == nil || *t.Public) && (t.ReviewStatus == "" || t.ReviewStatus == "approved") && (t.BuildStatus == "" || t.BuildStatus == "ready") && len(t.Manifest.Env) == 0 && len(t.Manifest.Secrets) == 0
}

// Guest cookies are unguessable bearer identities, not a shared anonymous account.
func (s *Server) guestEndpoint(w http.ResponseWriter, r *http.Request) {
	// This transport is reserved for same-origin browser interactions. External API access uses /api/v1 authentication.
	browserSource := r.Header.Get("Origin")
	if browserSource == "" {
		browserSource = r.Referer()
	}
	source, err := url.Parse(browserSource)
	if err != nil || source.Host != r.Host || (source.Scheme != "http" && source.Scheme != "https") || (r.Header.Get("Sec-Fetch-Site") != "" && r.Header.Get("Sec-Fetch-Site") != "same-origin") {
		fail(w, 403, "匿名调用仅支持站内网页，请通过 API Key 或 OAuth 使用 API")
		return
	}
	if r.Method != "GET" {
		if origin := r.Header.Get("Origin"); origin != "" {
			u, e := url.Parse(origin)
			if e != nil || u.Host != r.Host {
				fail(w, 403, "跨站请求不被允许")
				return
			}
		}
		if !s.accountRate(w) {
			return
		}
	}
	token := ""
	if c, e := r.Cookie("tooldeck_guest"); e == nil && regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(c.Value) {
		token = c.Value
	}
	if token == "" {
		token = ID("") + ID("")
		http.SetCookie(w, &http.Cookie{Name: "tooldeck_guest", Value: token, Path: "/api/public/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: 86400 * 7})
	}
	p := Principal{Guest: true, ID: "guest:" + hash(token), Tools: []string{"*"}}
	path := strings.TrimPrefix(r.URL.Path, "/api/public/")
	parts := strings.Split(path, "/")
	switch {
	case path == "tools" && r.Method == "GET":
		list := []Tool{}
		s.store.Lock()
		for _, t := range s.store.State.Tools {
			if guestTool(t) {
				t.BuildLog = ""
				t.BuildError = ""
				t.BuildImage = ""
				t.Artifact = ""
				list = append(list, t)
			}
		}
		s.store.Unlock()
		sort.Slice(list, func(i, j int) bool { return list[i].Created.After(list[j].Created) })
		jsonResponse(w, 200, list)
	case len(parts) == 3 && parts[0] == "tools" && parts[2] == "runs" && r.Method == "POST":
		s.createRun(w, r, p, parts[1])
	case len(parts) == 3 && parts[0] == "runs" && parts[2] == "events" && r.Method == "GET":
		s.streamRun(w, r, p, parts[1])
	case parts[0] == "runs" && ((len(parts) == 2 && r.Method == "GET") || (len(parts) == 3 && parts[2] == "cancel" && r.Method == "POST")):
		s.runEndpoint(w, r, p, parts)
	case path == "files" && r.Method == "POST":
		s.uploadFile(w, r, p)
	case len(parts) == 2 && parts[0] == "files" && r.Method == "GET":
		s.download(w, r, p, parts[1])
	default:
		fail(w, 404, "not found")
	}
}

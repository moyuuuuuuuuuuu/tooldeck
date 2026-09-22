package platform

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecurityReservedEnvironmentAcrossManifestAndStoredTools(t *testing.T) {
	for _, name := range []string{"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG", "DOCKER_API_VERSION", "DOCKER_TLS_VERIFY", "LD_LIBRARY_PATH", "BASH_ENV", "GODEBUG", "GOGC", "SSL_CERT_FILE"} {
		t.Run(name, func(t *testing.T) {
			var manifest Manifest
			if err := json.Unmarshal([]byte(manifestJSON), &manifest); err != nil {
				t.Fatal(err)
			}
			manifest.Env = []EnvField{{Name: name}}
			if err := manifest.Validate(); err == nil {
				t.Fatal("unsafe tool environment accepted")
			}
			// Legacy definitions must be rejected even with a per-run value override.
			server := &Server{store: &Store{}}
			if _, _, err := server.toolEnvironmentWithOverrides(Tool{Manifest: manifest}, "alice", map[string]string{name: "attacker-controlled"}); err == nil {
				t.Fatal("legacy environment accepted at execution boundary")
			}
			manifest.Env = nil
			manifest.Secrets = []string{name}
			if err := manifest.Validate(); err == nil {
				t.Fatal("unsafe platform secret name accepted")
			}
		})
	}
	if err := validEnv([]EnvField{{Name: "COZE_API_TOKEN"}, {Name: "API_BASE_URL"}}); err != nil {
		t.Fatal(err)
	}
}

func TestSecurityCallbacksRejectReservedNetworksBeforeDial(t *testing.T) {
	transport := callbackHTTPClient().Transport.(*http.Transport)
	for _, host := range []string{"100.64.0.1", "100.100.100.200", "198.18.0.1", "0.1.2.3", "127.0.0.1", "::ffff:100.100.100.200", "fc00::1"} {
		t.Run(host, func(t *testing.T) {
			if validateCallbackURL("https://"+net.JoinHostPort(host, "443")+"/result") == nil {
				t.Fatal("reserved callback URL accepted")
			}
			conn, err := transport.DialContext(context.Background(), "tcp", net.JoinHostPort(host, "443"))
			if conn != nil {
				conn.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "private or reserved") {
				t.Fatalf("target was not rejected before a network connection: %v", err)
			}
		})
	}
	if !publicCallbackIP(net.ParseIP("8.8.8.8")) || !publicCallbackIP(net.ParseIP("2606:4700:4700::1111")) {
		t.Fatal("public addresses blocked")
	}
}

func TestSecurityPackageManifestBoundsAndTrailingData(t *testing.T) {
	for _, test := range []struct {
		name, manifest string
		valid          bool
	}{
		{"valid", manifestJSON + "\n", true},
		{"second value", manifestJSON + `{}`, false},
		{"trailing garbage", manifestJSON + `garbage`, false},
		{"oversized", manifestJSON + strings.Repeat(" ", 128<<10), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var data bytes.Buffer
			archive := zip.NewWriter(&data)
			for name, content := range map[string]string{"tooldeck.json": test.manifest, "main.py": "print(1)"} {
				writer, err := archive.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = writer.Write([]byte(content)); err != nil {
					t.Fatal(err)
				}
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			path := filepath.Join(root, "package.zip")
			if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := ExtractPackage(path, filepath.Join(root, "extracted"))
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
			if test.name == "oversized" {
				if _, err := os.Stat(filepath.Join(root, "extracted")); !os.IsNotExist(err) {
					t.Fatal("oversized manifest was extracted before rejection")
				}
			}
		})
	}
}

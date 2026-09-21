package platform

import (
	"testing"
	"time"
)

func TestCatalogToolsKeepsLatestPublishedVersion(t *testing.T) {
	now := time.Now()
	tool := func(id, name, version, review, build string, created time.Time) Tool {
		public := true
		return Tool{ID: id, Owner: "alice", Public: &public, ReviewStatus: review, BuildStatus: build, Created: created, Manifest: Manifest{Name: name, Version: version}}
	}
	tools := []Tool{
		tool("v1", "demo", "1.0.0", "approved", "ready", now),
		tool("v2-pending", "demo", "2.0.0", "pending", "ready", now.Add(time.Minute)),
		tool("v3-building", "demo", "3.0.0", "draft", "building", now.Add(2*time.Minute)),
		tool("other", "other", "1.0.0", "approved", "ready", now.Add(3*time.Minute)),
	}
	sameNameOtherAuthor := tool("bob-demo", "demo", "1.0.0", "approved", "ready", now.Add(4*time.Minute))
	sameNameOtherAuthor.Owner = "bob"
	tools = append(tools, sameNameOtherAuthor)
	got := catalogTools(tools)
	if len(got) != 3 || got[0].ID != "bob-demo" || got[1].ID != "other" || got[2].ID != "v1" {
		t.Fatalf("unexpected catalog: %#v", got)
	}
	tools[1].ReviewStatus = "approved"
	got = catalogTools(tools)
	if len(got) != 3 || got[2].ID != "v2-pending" {
		t.Fatalf("approved version did not replace the old version: %#v", got)
	}
	tools[1].Withdrawn = true
	got = catalogTools(tools)
	if len(got) != 3 || got[2].ID != "v1" {
		t.Fatalf("catalog did not fall back after withdrawal: %#v", got)
	}
}

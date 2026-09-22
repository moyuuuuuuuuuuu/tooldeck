package platform

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"
)

type queueConfig struct{ Workers, PerUser, Builds int }

func loadQueueConfig() (queueConfig, error) {
	q := queueConfig{2, 2, 1}
	for name, dst := range map[string]*int{"TOOLDECK_WORKER_CONCURRENCY": &q.Workers, "TOOLDECK_USER_CONCURRENCY": &q.PerUser, "TOOLDECK_BUILD_CONCURRENCY": &q.Builds} {
		if raw := os.Getenv(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > 64 {
				return q, fmt.Errorf("%s must be 1..64", name)
			}
			*dst = n
		}
	}
	return q, nil
}

func toolFamily(t Tool) string { return toolOwner(t) + ":" + t.Manifest.Name }

// The store lock covers selection, quotas and durable claim. activeRuns keeps
// the slot occupied until execute returns, even after a cancellation response.
func (s *Server) claimRun() *Run {
	s.store.Lock()
	defer s.store.Unlock()
	users, tools := map[string]int{}, map[string]int{}
	active := map[string]Run{}
	for id, r := range s.activeRuns {
		active[id] = r
	}
	queued := []Run{}
	for id, r := range s.store.State.Runs {
		if r.Status == "running" || r.Status == "canceling" {
			active[id] = r
		}
		if r.Status == "queued" {
			queued = append(queued, r)
		}
	}
	if len(queued) == 0 {
		return nil
	}
	if len(active) >= s.queue.Workers {
		s.limitHits["global"]++
		return nil
	}
	for _, r := range active {
		users[r.Owner]++
		tools[toolFamily(s.store.State.Tools[r.ToolID])]++
	}
	sort.Slice(queued, func(i, j int) bool {
		if queued[i].Created.Equal(queued[j].Created) {
			return queued[i].ID < queued[j].ID
		}
		return queued[i].Created.Before(queued[j].Created)
	})
	// Use the strictest configured version limit across a tool family.
	caps := map[string]int{}
	for _, t := range s.store.State.Tools {
		n := t.Manifest.Execution.Concurrency
		k := toolFamily(t)
		if n > 0 && (caps[k] == 0 || n < caps[k]) {
			caps[k] = n
		}
	}
	for _, r := range queued {
		if users[r.Owner] >= s.queue.PerUser {
			s.limitHits["user"]++
			continue
		}
		family := toolFamily(s.store.State.Tools[r.ToolID])
		if cap := caps[family]; cap > 0 && tools[family] >= cap {
			s.limitHits["tool"]++
			continue
		}
		old := r
		now := time.Now()
		r.Started = &now
		r.Status = "running"
		s.store.State.Runs[r.ID] = r
		if err := s.store.save(); err != nil {
			s.store.State.Runs[r.ID] = old
			return nil
		}
		s.activeRuns[r.ID] = r
		return &r
	}
	return nil
}

func (s *Server) queueMetrics() map[string]any {
	s.store.Lock()
	defer s.store.Unlock()
	queued, running, builds, buildQueue := 0, 0, 0, 0
	var oldest, waitTotal int64
	started := 0
	for _, r := range s.store.State.Runs {
		if r.Status == "queued" {
			queued++
			age := time.Since(r.Created).Milliseconds()
			if age > oldest {
				oldest = age
			}
		}
		if r.Status == "running" || r.Status == "canceling" {
			running++
		}
		if r.Started != nil {
			waitTotal += r.Started.Sub(r.Created).Milliseconds()
			started++
		}
	}
	for _, t := range s.store.State.Tools {
		if t.BuildStatus == "building" {
			builds++
		}
		if t.BuildStatus == "queued" {
			buildQueue++
		}
	}
	hits := map[string]uint64{}
	for k, v := range s.limitHits {
		hits[k] = v
	}
	mean := int64(0)
	if started > 0 {
		mean = waitTotal / int64(started)
	}
	return map[string]any{"queued": queued, "running": running, "occupied_slots": len(s.activeRuns), "building": builds, "build_queued": buildQueue, "oldest_wait_ms": oldest, "mean_wait_ms": mean, "limit_hits": hits}
}

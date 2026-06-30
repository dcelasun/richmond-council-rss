package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/dcelasun/richmond-council-rss/internal/model"
)

// Memory is a concurrency-safe in-memory Store.
type Memory struct {
	mu      sync.RWMutex
	byID    map[string]model.Article
	ordered []model.Article // sorted by Published descending
	lastMod time.Time
}

func NewMemory() *Memory {
	return &Memory{byID: make(map[string]model.Article)}
}

func (m *Memory) Has(_ context.Context, id string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.byID[id]
	return ok, nil
}

func (m *Memory) Upsert(_ context.Context, a model.Article) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[a.ID] = a
	if a.FetchedAt.After(m.lastMod) {
		m.lastMod = a.FetchedAt
	}
	m.rebuild()
	return nil
}

// rebuild rebuilds the ordered slice under the write lock. Volumes are small,
// so a full re-sort on each upsert is fine.
func (m *Memory) rebuild() {
	m.ordered = m.ordered[:0]
	for _, a := range m.byID {
		m.ordered = append(m.ordered, a)
	}
	sort.SliceStable(m.ordered, func(i, j int) bool {
		return m.ordered[i].Published.After(m.ordered[j].Published)
	})
}

func (m *Memory) Page(_ context.Context, limit, offset int) ([]model.Article, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if offset >= len(m.ordered) || offset < 0 {
		return nil, nil
	}
	end := offset + limit
	if limit <= 0 || end > len(m.ordered) {
		end = len(m.ordered)
	}
	out := make([]model.Article, end-offset)
	copy(out, m.ordered[offset:end])
	return out, nil
}

func (m *Memory) Count(_ context.Context) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.byID), nil
}

func (m *Memory) LastModified(_ context.Context) (time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastMod, nil
}

func (m *Memory) Close() error { return nil }

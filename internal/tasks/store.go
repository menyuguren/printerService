package tasks

import (
	"sync"
	"time"
)

type Status string

const (
	StatusReceived  Status = "received"
	StatusSubmitted Status = "submitted"
	StatusFailed    Status = "failed"
)

type Task struct {
	ID        string    `json:"id"`
	SourceIP  string    `json:"source_ip"`
	Printer   string    `json:"printer"`
	Size      int64     `json:"size"`
	Status    Status    `json:"status"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store struct {
	mu    sync.Mutex
	limit int
	items []Task
}

func NewStore(limit int) *Store {
	if limit <= 0 {
		limit = 20
	}
	return &Store{limit: limit}
}

func (s *Store) Add(task Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}
	s.items = append([]Task{task}, s.items...)
	if len(s.items) > s.limit {
		s.items = s.items[:s.limit]
	}
}

func (s *Store) Update(id string, status Status, errText string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].Status = status
			s.items[i].Error = errText
			s.items[i].UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

func (s *Store) Recent() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Task, len(s.items))
	copy(out, s.items)
	return out
}

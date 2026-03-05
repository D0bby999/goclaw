package queue

import "sync"

// Queue is a thread-safe crawl request queue with deduplication.
type Queue struct {
	mu        sync.Mutex
	items     []CrawlRequest
	seen      map[string]bool
	completed int
	failed    int
	strategy  Strategy
}

// New creates a Queue with the given traversal strategy.
func New(strategy Strategy) *Queue {
	return &Queue{
		seen:     make(map[string]bool),
		strategy: strategy,
	}
}

// Add enqueues req if not already seen. Returns true if the item was a duplicate
// (already seen), false if it was newly added.
func (q *Queue) Add(req CrawlRequest) bool {
	key := req.UniqueKey
	if key == "" {
		key = NormalizeKey(req.URL)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.seen[key] {
		return true
	}
	q.seen[key] = true
	req.UniqueKey = key
	q.items = append(q.items, req)
	return false
}

// AddBatch enqueues multiple requests and returns the count of newly added items.
func (q *Queue) AddBatch(reqs []CrawlRequest) int {
	added := 0
	for _, r := range reqs {
		if !q.Add(r) {
			added++
		}
	}
	return added
}

// Next pops the next request according to strategy.
// BFS pops from the front; DFS pops from the back.
// Returns false if the queue is empty.
func (q *Queue) Next() (CrawlRequest, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return CrawlRequest{}, false
	}

	var req CrawlRequest
	if q.strategy == DFS {
		last := len(q.items) - 1
		req = q.items[last]
		q.items = q.items[:last]
	} else {
		req = q.items[0]
		q.items = q.items[1:]
	}
	return req, true
}

// MarkCompleted increments the completed counter for the given key.
func (q *Queue) MarkCompleted(_ string) {
	q.mu.Lock()
	q.completed++
	q.mu.Unlock()
}

// MarkFailed increments the failed counter for the given key.
func (q *Queue) MarkFailed(_ string) {
	q.mu.Lock()
	q.failed++
	q.mu.Unlock()
}

// Stats returns a snapshot of queue counters.
func (q *Queue) Stats() Stats {
	q.mu.Lock()
	defer q.mu.Unlock()
	return Stats{
		Pending:   len(q.items),
		Completed: q.completed,
		Failed:    q.failed,
	}
}

// IsEmpty returns true when no items are pending.
func (q *Queue) IsEmpty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items) == 0
}

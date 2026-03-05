package queue

type Strategy int

const (
	BFS Strategy = iota
	DFS
)

type CrawlRequest struct {
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers,omitempty"`
	UserData   map[string]any    `json:"user_data,omitempty"`
	UniqueKey  string            `json:"unique_key"`
	RetryCount int               `json:"retry_count"`
	MaxRetries int               `json:"max_retries"`
	NoRetry    bool              `json:"no_retry"`
	Label      string            `json:"label,omitempty"`
	Depth      int               `json:"depth"`
}

type Stats struct {
	Pending   int `json:"pending"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

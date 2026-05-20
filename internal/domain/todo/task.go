package todo

type Task struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Start        *int64   `json:"start"`
	Deadline     *int64   `json:"deadline"`
	Status       string   `json:"status"`
	Priority     *string  `json:"priority"`
	Tags         []string `json:"tags"`
	Type         *string  `json:"type"`
	TaskDoneTime *int64   `json:"taskDoneTime"`
	CreatedAt    int64    `json:"createdAt"`
	UpdatedAt    int64    `json:"updatedAt"`
	DeletedAt    *int64   `json:"deletedAt"`
	Version      int64    `json:"version"`
}

type SyncEvent struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"createdAt"`
	Entity    string `json:"entity"`
	EntityID  string `json:"entityId"`
	Type      string `json:"type"`
	Payload   Task   `json:"payload"`
}

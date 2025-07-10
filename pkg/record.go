package pkg

type Record struct {
	Key       string `json:"key"`
	Topic     string `json:"topic"`
	Content   string `json:"content"`
	UpdatedAt int64  `json:"updated_at"`
	CreatedAt int64  `json:"created_at"`
}

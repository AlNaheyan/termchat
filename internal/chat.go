package internal

type ChatMessage struct {
	Room string `json:"room"`
	User string `json:"user"`
	Body string `json:"body"`
	Ts   int64  `json:"ts"`
}

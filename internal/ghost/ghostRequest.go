package ghost

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Request struct {
	Uuid          uuid.UUID
	CreatedAt     time.Time
	ExecuteAt     time.Time
	Method        string
	Url           string
	NotifyUrl     string
	Headers       map[string][]string
	NotifyHeaders map[string][]string
	Body          []byte
}

func NewRequest() *Request {
	now := time.Now()
	delay := 3 * time.Second

	return &Request{
		Uuid:          uuid.New(),
		CreatedAt:     now,
		ExecuteAt:     now.Add(delay),
		Headers:       make(map[string][]string),
		NotifyHeaders: make(map[string][]string),
		Method:        "GET",
		Url:           "http://localhost:8112/status",
	}
}

func (req *Request) String() string {
	return fmt.Sprintf("<%s> (%s) %s @ %s",
		req.Uuid.String(),
		req.Method,
		req.Url,
		req.ExecuteAt.Format(TimeFormat))
}

func (req *Request) UrlString() string {
	return fmt.Sprintf("(%s) %s",
		req.Method,
		req.Url)
}

func (req *Request) DurationRemaining() time.Duration {
	return req.ExecuteAt.Sub(time.Now())
}

func (req *Request) ShouldExecute() bool {
	return req.DurationRemaining() <= 0
}

package ghost

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Request struct {
	Uuid      uuid.UUID
	CreatedAt time.Time
	ExecuteAt time.Time
	Method    string
	Url       string
	NotifyUrl string
	Headers   map[string][]string
	Body      []byte
	Executing bool
}

func NewRequest() *Request {
	now := time.Now()
	delay := 3 * time.Second

	return &Request{
		Uuid:      uuid.New(),
		CreatedAt: now,
		ExecuteAt: now.Add(delay),
		Headers:   make(map[string][]string),
		Method:    "GET",
		Url:       "http://localhost:8112/status",
		NotifyUrl: "http://localhost:8112/status",
		Executing: false,
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
	if req.Executing {
		return false
	}

	return req.DurationRemaining() <= 0
}

func (req *Request) notifyFailure() {

	uuidStr := req.Uuid.String()
	bodyContent := fmt.Sprintf("Failed to send request with id [%s]\n", uuidStr)

	if len(req.NotifyUrl) > 0 {
		http.Post(req.NotifyUrl, "text/plain", strings.NewReader(bodyContent))
	}
}

func (req *Request) notifySuccess() {

	uuidStr := req.Uuid.String()
	bodyContent := fmt.Sprintf("Successfully sent request with id [%s]\n", uuidStr)

	if len(req.NotifyUrl) > 0 {
		http.Post(req.NotifyUrl, "text/plain", strings.NewReader(bodyContent))
	}
}

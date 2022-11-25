package ghost

import (
	"bytes"
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
		Url:       "http://localhost:8090/status",
		NotifyUrl: "http://localhost:8090/status",
	}
}

func (req *Request) String() string {
	return fmt.Sprintf("<%s> (%s) %s @ %s",
		req.Uuid.String(),
		req.Method,
		req.Url,
		req.ExecuteAt.Format(TimeFormat))
}

func (req *Request) DurationRemaining() time.Duration {
	return req.ExecuteAt.Sub(time.Now())
}

func (req *Request) ShouldExecute() bool {
	return req.DurationRemaining() <= 0
}

func notifyFailure(req *Request) {

	uuidStr := req.Uuid.String()
	bodyContent := fmt.Sprintf("Failed to send request with id [%s]\n", uuidStr)

	if len(req.NotifyUrl) > 0 {
		http.Post(req.NotifyUrl, "text/plain", strings.NewReader(bodyContent))
	}
}

func notifySuccess(req *Request) {

	uuidStr := req.Uuid.String()
	bodyContent := fmt.Sprintf("Successfully sent request with id [%s]\n", uuidStr)

	if len(req.NotifyUrl) > 0 {
		http.Post(req.NotifyUrl, "text/plain", strings.NewReader(bodyContent))
	}
}

func (ghostRequest *Request) Execute(e *Engine) error {
	defer e.removeFromMap(ghostRequest.Uuid)

	bodyReader := bytes.NewReader(ghostRequest.Body)
	req, err := http.NewRequest(ghostRequest.Method, ghostRequest.Url, bodyReader)

	if err != nil {
		notifyFailure(ghostRequest)
		e.requestsErrorCount++
		return err
	}

	for key, arr := range ghostRequest.Headers {
		for _, val := range arr {
			req.Header.Add(key, val)
		}
	}

	_, err = http.DefaultClient.Do(req)
	if err != nil {
		notifyFailure(ghostRequest)
		e.requestsErrorCount++
		return err
	}

	notifySuccess(ghostRequest)
	e.requestsServedCount++

	fmt.Printf("Sent request %s\n", ghostRequest.String())

	return nil
}

package ghost

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"encoding/gob"

	"github.com/google/uuid"
)

var singleton *Engine
var TimeFormat string = "2006-01-02 15:04:05"

func GetEngine() *Engine {
	return singleton
}

type Engine struct {
	startupTime             time.Time
	requestsRegisteredCount uint
	requestsServedCount     uint
	requestsErrorCount      uint
	requests                chan *Request
	requestMap              map[uuid.UUID]*Request
	requeueTimeout          time.Duration
}

func StartEngine(capacity int) *Engine {
	singleton = &Engine{
		requests:       make(chan *Request, capacity),
		requestMap:     make(map[uuid.UUID]*Request),
		startupTime:    time.Now(),
		requeueTimeout: time.Second,
	}

	return singleton
}

func (e *Engine) Run() error {
	for {
		// if the channel has a non-zero length, we should dequeue and check each request
		// to see if it should be executed or not - if not, we can add it back onto the
		// channel for the next check
		for i := 0; i < len(e.requests); i++ {
			ghostRequest := <-e.requests

			if ghostRequest.ShouldExecute() == false {
				e.requests <- ghostRequest
				continue
			}

			// request should be executed
			go ghostRequest.Execute(e)
		}

		// we don't need to destroy our cpu by constantly requeuing the request channel,
		// so just check every now and then
		time.Sleep(e.requeueTimeout)
	}
}

func (e *Engine) RegisterRequest(ghostRequest *Request) error {

	select {
	case e.requests <- ghostRequest: // Put request into channel, unless it's full
		e.requestMap[ghostRequest.Uuid] = ghostRequest
		e.requestsRegisteredCount++
	default:
		return errors.New("Engine capacity exceeded. \nDid not register request. \nGhost cannot register any further requests until one or more are fulfilled. \n")
	}

	return nil
}

func (e Engine) RequestStatus(uuid uuid.UUID) string {
	request, exists := e.requestMap[uuid]

	if exists {
		str := fmt.Sprintf("UUID: %s\n", request.Uuid.String()) +
			fmt.Sprintf("Method: %s\n", request.Method) +
			fmt.Sprintf("URL: %s\n", request.Url) +
			fmt.Sprintf("In queue since: %s\n", request.CreatedAt.Format(TimeFormat)) +
			fmt.Sprintf("Will execute at: %s\n", request.ExecuteAt.Format(TimeFormat))
		return str
	}

	return "Not Found"
}

func (e Engine) removeFromMap(uuid uuid.UUID) {
	delete(e.requestMap, uuid)
}

type EngineStatus struct {
	UptimeMinutes             uint
	StartupTime               time.Time
	RequeueTimeout            time.Duration
	RequestsRegisteredCount   uint
	RequestsServedCount       uint
	RequestsErrorCount        uint
	RequestsRegisteredPerHour uint
	RequestsServedPerHour     uint
	RequestsErrorsPerHour     uint
	QueueCapacity             int
	QueueLength               int
}

func (e *Engine) Status() EngineStatus {

	minsSinceStart := time.Now().Sub(e.startupTime).Minutes()

	var requestsRegisteredPerHour uint
	var requestsServedPerHour uint
	var requestsErrorPerHour uint

	// avoid divide by zero errors
	if minsSinceStart > 0 {
		requestsRegisteredPerHour = uint(float64(e.requestsRegisteredCount) / (minsSinceStart / 60))
		requestsServedPerHour = uint(float64(e.requestsServedCount) / (minsSinceStart / 60))
		requestsErrorPerHour = uint(float64(e.requestsErrorCount) / (minsSinceStart / 60))
	}

	return EngineStatus{
		UptimeMinutes:             uint(minsSinceStart),
		StartupTime:               e.startupTime,
		RequeueTimeout:            e.requeueTimeout,
		RequestsRegisteredCount:   e.requestsRegisteredCount,
		RequestsServedCount:       e.requestsServedCount,
		RequestsErrorCount:        e.requestsErrorCount,
		RequestsRegisteredPerHour: requestsRegisteredPerHour,
		RequestsServedPerHour:     requestsServedPerHour,
		RequestsErrorsPerHour:     requestsErrorPerHour,
		QueueCapacity:             cap(e.requests),
		QueueLength:               len(e.requests),
	}
}

type SaveData struct {
	Requests []Request
}

func (e *Engine) Save() error {

	ghostSaveData := SaveData{}
	ghostSaveData.Requests = make([]Request, 0, cap(e.requests))

	close(e.requests)
	for r := range e.requests {
		ghostSaveData.Requests = append(ghostSaveData.Requests, *r)
	}

	// if there's no requests pending, we dont need to save anything
	if len(ghostSaveData.Requests) < 1 {
		return nil
	}

	fmt.Print("Saving pending requests to file... ")

	file, err := os.Create("./ghostdb")
	if err != nil {
		file.Close()
		return err
	}

	encoder := gob.NewEncoder(file)
	encoder.Encode(&ghostSaveData)

	fmt.Println("Done!")

	return nil
}

func (e *Engine) Load() error {
	fileBytes, err := os.ReadFile("./ghostdb")
	if err != nil {
		return err
	}

	fmt.Print("Loading pending requests from file... ")

	ghostSaveData := SaveData{}

	encoder := gob.NewDecoder(bytes.NewReader(fileBytes))
	encoder.Decode(&ghostSaveData)

	for _, req := range ghostSaveData.Requests {
		// if the loaded request is more than 12 hours out of date,
		// it may be a worse idea to send it at this point
		if req.DurationRemaining().Hours() < 12 {
			e.RegisterRequest(&req)
		}
	}

	os.Remove("./ghostdb")

	fmt.Println("Done!")

	return nil
}

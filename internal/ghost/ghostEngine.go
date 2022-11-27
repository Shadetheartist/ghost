package ghost

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
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
	incomingRequests        chan *Request
	completedRequests       chan *Request
	requestMap              map[uuid.UUID]*Request
	maxPendingRequests      int
	requeueTimeout          time.Duration
	ticker                  *time.Ticker
	done                    chan bool
	dbFileLocation          string
}

func NewEngine(maxPendingRequests int, fluxCapacity int, requeueTimeout time.Duration) *Engine {
	singleton = &Engine{
		incomingRequests:   make(chan *Request, fluxCapacity),
		completedRequests:  make(chan *Request, fluxCapacity),
		requestMap:         make(map[uuid.UUID]*Request),
		startupTime:        time.Now(),
		requeueTimeout:     requeueTimeout,
		ticker:             time.NewTicker(requeueTimeout),
		done:               make(chan bool),
		maxPendingRequests: maxPendingRequests,
		dbFileLocation:     "./ghostdb",
	}

	return singleton
}

func (e *Engine) Halt() {
	e.done <- true
	close(e.incomingRequests)
	close(e.completedRequests)
	e.ticker.Stop()
}

func (e *Engine) Run() {
	for {
		select {
		case <-e.done:
			return

		case ghostRequest := <-e.incomingRequests:
			if len(e.requestMap) < e.maxPendingRequests {
				e.requestMap[ghostRequest.Uuid] = ghostRequest
			} else {
				fmt.Printf("Maximum pending requests reached. Ignoring request %s\n", ghostRequest.Uuid.String())
			}

		case ghostRequest := <-e.completedRequests:
			delete(e.requestMap, ghostRequest.Uuid)

		case <-e.ticker.C:
			for _, ghostRequest := range e.requestMap {
				if ghostRequest.ShouldExecute() {
					ghostRequest.Executing = true
					go e.Execute(ghostRequest)
				}
			}
		}
	}
}

func (e *Engine) RegisterRequest(ghostRequest *Request) error {

	select {
	case e.incomingRequests <- ghostRequest: // Put request into channel, unless it's full
		e.requestsRegisteredCount++
	default:
		return errors.New("Request flux queue capacity exceeded. Could not register request. \nGhost cannot register any further requests until one or more are fulfilled. \n")
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

type EngineStatus struct {
	UptimeMinutes           uint
	StartupTime             time.Time
	RequeueTimeout          time.Duration
	RequestsRegisteredCount uint
	RequestsServedCount     uint
	RequestsErrorCount      uint
	QueueCapacity           int
	QueueLength             int
	PendingRequestsCount    int
	PendingRequestsMax      int
}

func (e *Engine) Status() EngineStatus {

	minsSinceStart := time.Now().Sub(e.startupTime).Minutes()

	return EngineStatus{
		UptimeMinutes:           uint(minsSinceStart),
		StartupTime:             e.startupTime,
		RequeueTimeout:          e.requeueTimeout,
		RequestsRegisteredCount: e.requestsRegisteredCount,
		RequestsServedCount:     e.requestsServedCount,
		RequestsErrorCount:      e.requestsErrorCount,
		QueueCapacity:           cap(e.incomingRequests),
		QueueLength:             len(e.incomingRequests),
		PendingRequestsCount:    len(e.requestMap),
		PendingRequestsMax:      e.maxPendingRequests,
	}
}

type SaveData struct {
	Requests []Request
}

// saves pending requests to ghostdb file
func (e *Engine) Save() error {

	ghostSaveData := SaveData{}
	ghostSaveData.Requests = make([]Request, 0, len(e.requestMap))

	for _, r := range e.requestMap {
		ghostSaveData.Requests = append(ghostSaveData.Requests, *r)
	}

	// if there's no requests pending, we dont need to save anything
	if len(ghostSaveData.Requests) < 1 {
		return nil
	}

	fmt.Print("Saving pending requests to file... ")

	file, err := os.Create(e.dbFileLocation)
	if err != nil {
		file.Close()
		return err
	}

	encoder := gob.NewEncoder(file)
	encoder.Encode(&ghostSaveData)

	fmt.Println("Done!")

	return nil
}

// loads requests from ghostdb file and then deletes the file (if it exists)
func (e *Engine) Load() error {

	// if the file doesn't exist, we don't need to load it
	if _, err := os.Stat(e.dbFileLocation); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	fileBytes, err := os.ReadFile(e.dbFileLocation)
	if err != nil {
		return err
	}

	fmt.Print("Loading pending requests from file... ")

	ghostSaveData := SaveData{}

	encoder := gob.NewDecoder(bytes.NewReader(fileBytes))
	encoder.Decode(&ghostSaveData)

	for idx := range ghostSaveData.Requests {
		err := e.RegisterRequest(&ghostSaveData.Requests[idx])
		if err != nil {
			fmt.Printf("Err registering loaded request: %s\n", err.Error())
		}
	}

	os.Remove("./ghostdb")

	fmt.Println("Done!")

	return nil
}

func (e *Engine) completeRequest(ghostRequest *Request) {
	e.completedRequests <- ghostRequest
}

func (e *Engine) Execute(ghostRequest *Request) {
	defer e.completeRequest(ghostRequest)

	bodyReader := bytes.NewReader(ghostRequest.Body)
	req, err := http.NewRequest(ghostRequest.Method, ghostRequest.Url, bodyReader)

	if err != nil {
		fmt.Printf("Failed Sending Request %s\n", err.Error())
		ghostRequest.notifyFailure()
		e.requestsErrorCount++
		return
	}

	for key, arr := range ghostRequest.Headers {
		for _, val := range arr {
			req.Header.Add(key, val)
		}
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Failed Sending Request %s\n", err.Error())
		ghostRequest.notifyFailure()
		e.requestsErrorCount++
		return
	}

	ghostRequest.notifySuccess()
	e.requestsServedCount++

	fmt.Printf("Sent request %s [%d]\n", ghostRequest.String(), response.StatusCode)
}

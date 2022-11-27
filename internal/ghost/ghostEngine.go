package ghost

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var singleton *Engine
var TimeFormat string = "2006-01-02 15:04:05"

func GetEngine() *Engine {
	return singleton
}

type Notification struct {
	ghostRequest *Request
	httpResponse *http.Response
	err          error
}

type Engine struct {
	startupTime              time.Time
	requestsRegisteredCount  uint
	requestsServedCount      uint
	requestsErrorCount       uint
	notificationsServedCount uint
	notificationsErrorCount  uint
	incrementRequestsServed  chan int
	incrementRequestsErr     chan int
	incrementNotifyServed    chan int
	incrementNotifyErr       chan int
	incomingRequests         chan *Request
	activeRequests           chan *Request
	completeRequests         chan *Request
	activeNotifyRequests     chan *Notification
	notifyQueue              chan *Notification
	requestMap               map[uuid.UUID]*Request
	requeueTimeout           time.Duration
	ticker                   *time.Ticker
	done                     chan bool
	dbFileLocation           string
}

func NewEngine(capacity int, maxActiveRequests int, maxActiveNotifications int, requeueTimeout time.Duration) *Engine {
	singleton = &Engine{
		incomingRequests:        make(chan *Request, capacity),
		completeRequests:        make(chan *Request, maxActiveRequests),
		activeRequests:          make(chan *Request, maxActiveRequests),
		activeNotifyRequests:    make(chan *Notification, maxActiveNotifications),
		notifyQueue:             make(chan *Notification, capacity),
		incrementRequestsServed: make(chan int, maxActiveRequests),
		incrementRequestsErr:    make(chan int, maxActiveRequests),
		incrementNotifyServed:   make(chan int, maxActiveNotifications),
		incrementNotifyErr:      make(chan int, maxActiveNotifications),
		requestMap:              make(map[uuid.UUID]*Request),
		startupTime:             time.Now(),
		requeueTimeout:          requeueTimeout,
		ticker:                  time.NewTicker(requeueTimeout),
		done:                    make(chan bool),
		dbFileLocation:          "./ghostdb",
	}

	return singleton
}

func (e *Engine) Halt() {
	e.done <- true
	close(e.incomingRequests)
	close(e.completeRequests)
	close(e.activeRequests)
	close(e.notifyQueue)
	close(e.activeNotifyRequests)
	close(e.incrementRequestsServed)
	close(e.incrementRequestsErr)
	e.ticker.Stop()
}

func (e *Engine) Run() {

	go e.heartbeat()

	for {
		select {
		case <-e.done:
			return

		case n := <-e.incrementRequestsServed:
			e.requestsServedCount = e.requestsServedCount + uint(n)

		case n := <-e.incrementRequestsErr:
			e.requestsErrorCount = e.requestsErrorCount + uint(n)

		case n := <-e.incrementNotifyServed:
			e.notificationsServedCount = e.notificationsServedCount + uint(n)

		case n := <-e.incrementNotifyErr:
			e.notificationsErrorCount = e.notificationsErrorCount + uint(n)

		case ghostRequest := <-e.completeRequests:
			delete(e.requestMap, ghostRequest.Uuid)

		case notification := <-e.notifyQueue:
			e.activeNotifyRequests <- notification
			go e.sendNotification(notification)
		}
	}
}

func (e *Engine) heartbeat() {

	for {
		select {
		case <-e.done:
			return

		case <-e.ticker.C:
			for i := 0; i < len(e.incomingRequests); i++ {
				ghostRequest := <-e.incomingRequests

				if ghostRequest.ShouldExecute() == false {
					e.incomingRequests <- ghostRequest
					continue
				}

				select {
				case e.activeRequests <- ghostRequest:
					go e.Execute(ghostRequest)
				default:
					e.incomingRequests <- ghostRequest
					continue
				}

			}
		}
	}
}

func (e *Engine) RegisterRequest(ghostRequest *Request) error {

	select {
	case e.incomingRequests <- ghostRequest: // Put request into channel, unless it's full
		e.requestMap[ghostRequest.Uuid] = ghostRequest
		e.requestsRegisteredCount++
	default:
		return errors.New("Request queue capacity exceeded. Could not register request. \nGhost cannot register any further requests until one or more are fulfilled. \n")
	}

	return nil
}

func (e *Engine) completeRequest(ghostRequest *Request, response *http.Response, err error) {
	<-e.activeRequests

	if err != nil {
		log.Printf("Failed Sending Request %s\n", err.Error())
		e.incrementRequestsErr <- 1
	} else {
		e.incrementRequestsServed <- 1
		log.Printf("Sent request %s [%d]\n", ghostRequest.String(), response.StatusCode)
	}

	if ghostRequest.NotifyUrl != "" {
		e.notifyQueue <- &Notification{
			ghostRequest: ghostRequest,
			httpResponse: response,
			err:          err,
		}
	} else {
		e.completeRequests <- ghostRequest
	}
}

func (e *Engine) Execute(ghostRequest *Request) {

	bodyReader := bytes.NewReader(ghostRequest.Body)
	req, err := http.NewRequest(ghostRequest.Method, ghostRequest.Url, bodyReader)

	if err != nil {
		e.completeRequest(ghostRequest, nil, err)
		return
	}

	for key, arr := range ghostRequest.Headers {
		for _, val := range arr {
			req.Header.Add(key, val)
		}
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		e.completeRequest(ghostRequest, response, err)
		return
	}

	e.completeRequest(ghostRequest, response, nil)
}

func (e *Engine) sendNotification(notification *Notification) {

	var bodyBytes []byte
	var err error

	if notification.httpResponse != nil && notification.httpResponse.Body != nil {
		bodyBytes, err = ioutil.ReadAll(notification.httpResponse.Body)

		if err != nil {
			log.Printf(
				"Could not send notification to %s (could not read body content)\n",
				notification.ghostRequest.NotifyUrl)

			e.completeRequests <- notification.ghostRequest
			e.incrementNotifyErr <- 1
			<-e.activeNotifyRequests
			return
		}

		notification.httpResponse.Body.Close()
	}

	req, err := http.NewRequest("POST", notification.ghostRequest.NotifyUrl, bytes.NewReader(bodyBytes))

	if err != nil {
		log.Printf(
			"Could not send notification to %s (could not constuct request)\n",
			notification.ghostRequest.NotifyUrl)

		<-e.activeNotifyRequests
		e.completeRequests <- notification.ghostRequest
		e.incrementNotifyErr <- 1
		return
	}

	if notification.httpResponse != nil {
		req.Header.Set("Content-Type", notification.httpResponse.Header.Get("Content-Type"))
	}

	for key, arr := range notification.ghostRequest.NotifyHeaders {
		for _, val := range arr {
			req.Header.Add(key, val)
		}
	}

	response, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Printf(
			"Could not send notification to %s (%s)\n",
			notification.ghostRequest.NotifyUrl,
			err.Error())

		<-e.activeNotifyRequests
		e.incrementNotifyErr <- 1
		e.completeRequests <- notification.ghostRequest
		return
	}

	log.Printf("Sent notification for %s [%d]\n", notification.ghostRequest.String(), response.StatusCode)

	<-e.activeNotifyRequests
	e.completeRequests <- notification.ghostRequest
	e.incrementNotifyServed <- 1
}

package ghost

import (
	"time"

	"github.com/google/uuid"
)

func (e Engine) GetPendingRequest(uuid uuid.UUID) (*Request, bool) {
	request, exists := e.requestMap[uuid]
	return request, exists
}

type EngineStatus struct {
	UptimeMinutes               uint
	StartupTime                 time.Time
	RequeueTimeout              time.Duration
	RequestsRegisteredCount     uint
	RequestsServedCount         uint
	RequestsErrorCount          uint
	QueueCapacity               int
	QueueLength                 int
	ActiveRequestsCount         int
	ActiveRequestsCapacity      int
	ActiveNotificationsCount    int
	ActiveNotificationsCapacity int
	NotificationQueue           int
	NotificationQueueCapacity   int
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

		QueueCapacity: cap(e.incomingRequests),
		QueueLength:   len(e.incomingRequests),

		ActiveRequestsCount:    len(e.activeRequests),
		ActiveRequestsCapacity: cap(e.activeRequests),

		ActiveNotificationsCount:    len(e.activeNotifyRequests),
		ActiveNotificationsCapacity: cap(e.activeNotifyRequests),

		NotificationQueue:         len(e.notifyQueue),
		NotificationQueueCapacity: cap(e.notifyQueue),
	}
}

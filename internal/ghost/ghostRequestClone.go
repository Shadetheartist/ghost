package ghost

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func parseIsoTimeString(isoTimeStr string) (time.Time, error) {
	return time.Parse(time.RFC3339, isoTimeStr)
}

func getHeader(headerKey string, req *http.Request) (string, error) {
	header := req.Header.Get(headerKey)
	if header == "" {
		return "", errors.New(fmt.Sprintf("Header '%s' must be set.", headerKey))
	}

	return header, nil
}

func CloneHttpRequest(req *http.Request) (*Request, error) {

	ghostRequest := NewRequest()
	ghostRequest.Method = req.Method

	if header, err := getHeader("X-Ghost-Url", req); err == nil {
		ghostRequest.Url = header
	} else {
		return nil, err
	}

	header, _ := getHeader("X-Ghost-Notify-Url", req)
	ghostRequest.NotifyUrl = header

	notifyHeaderKey, _ := getHeader("X-Ghost-Notify-Header-Key", req)
	notifyHeaderVal, _ := getHeader("X-Ghost-Notify-Header-Value", req)

	if notifyHeaderKey != "" && notifyHeaderVal != "" {
		headerVals := make([]string, 1)
		headerVals[0] = notifyHeaderVal
		ghostRequest.NotifyHeaders[notifyHeaderKey] = headerVals
	}

	if header, err := getHeader("X-Ghost-Exec-At", req); err == nil {
		if header, err := parseIsoTimeString(header); err == nil {
			ghostRequest.ExecuteAt = header
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}

	if req.Body != nil {
		bodyBytes, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, errors.New("Failed cloning request body")
		}

		ghostRequest.Body = bodyBytes
	}

	for key, value := range req.Header {
		// lets remove the ghost parameter headers
		if strings.HasPrefix(key, "X-Ghost") {
			continue
		}
		ghostRequest.Headers[key] = value
	}

	ghostRequest.Headers["x-Ghosted"] = []string{}

	return ghostRequest, nil
}

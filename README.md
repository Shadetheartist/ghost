# Ghost

Ghost is a http request redirection utility server. 

Send Ghost a web request with some configuration parameters in the request headers, and it will clone that request and send it to your specified url at your specified time.

```
X-Ghost-Url: <the url to send this request to>
X-Ghost-Exec-At: <when to send the request (rfc 3339)>
```

```
Usage of ghost:
  -max-flux int
        The maximum capacity of the unprocessed request queue. (default 100)
  -max-pending int
        The maximum capacity of pending requests. (default 10000)
  -port int
        Set the port that the server will run on. (default 8112)
```
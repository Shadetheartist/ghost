# Ghost

Ghost is a http request redirection utility server. 

Send Ghost a web request with some configuration parameters in the request headers, and it will clone that request and send it to your specified url at your specified time.

```
X-Ghost-Url: <the url to send this request to>
X-Ghost-Exec-At: <when to send the request (rfc 3339)>
```

```
Usage of ghost:
  -active int
        The maximum capacity of active requests at any given moment. (default 16)
  -active-notifications int
        The maximum capacity of active notification requests at any given moment. (default 16)
  -capacity int
        The maximum capacity of the unprocessed request queue. (default 1024)
  -load
        If the ghostdb file is available, load from it.
  -port int
        Set the port that the server will run on. (default 8112)
```
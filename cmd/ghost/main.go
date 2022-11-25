package main

import (
	"context"
	"flag"
	"fmt"
	"internal/ghost"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
)

func handleClone(w http.ResponseWriter, req *http.Request) {

	ghostRequest, err := ghost.CloneHttpRequest(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Println(err.Error())
		fmt.Fprintf(w, err.Error())
		return
	}

	err = ghost.GetEngine().RegisterRequest(ghostRequest)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Fprintf(w, err.Error())
		return
	}

	ghostRequestStr := ghostRequest.String()
	logStr := fmt.Sprintf("cloned http request and registered %s\n", ghostRequestStr)
	fmt.Print(logStr)
	fmt.Fprintf(w, logStr)
}

func handleStatus(w http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)
	id, ok := vars["id"]
	if !ok {
		engineStatus := ghost.GetEngine().Status()
		statStr := fmt.Sprintf("Startup Time: %s (uptime: %d mimutes)\n",
			engineStatus.StartupTime.Format(ghost.TimeFormat),
			engineStatus.UptimeMinutes)
		reqRegStr := fmt.Sprintf("Requests Registered: %d (%d/h)\n",
			engineStatus.RequestsRegisteredCount,
			engineStatus.RequestsRegisteredPerHour)
		reqServStr := fmt.Sprintf("Requests Served: %d (%d/h)\n",
			engineStatus.RequestsServedCount,
			engineStatus.RequestsServedPerHour)
		reqErrStr := fmt.Sprintf("Request Errors: %d (%d/h)\n",
			engineStatus.RequestsErrorCount,
			engineStatus.RequestsErrorsPerHour)
		queueStr := fmt.Sprintf("Queue: %d/%d\n",
			engineStatus.QueueLength,
			engineStatus.QueueCapacity)
		fmt.Fprintf(w, statStr)
		fmt.Fprintf(w, reqRegStr)
		fmt.Fprintf(w, reqServStr)
		fmt.Fprintf(w, reqErrStr)
		fmt.Fprintf(w, queueStr)
	} else {
		uuid := uuid.MustParse(id)
		fmt.Fprintf(w, ghost.GetEngine().RequestStatus(uuid))
	}

}

func main() {

	port := flag.Int("port", 8090, "Set the port that the server will run on.")
	flag.Parse()

	serverAddress := fmt.Sprintf(":%d", *port)

	fmt.Println("Starting Ghost Server at", serverAddress)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// we need to reserve to buffer size 1, so the notifier are not blocked
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)

		<-c
		cancel()
	}()

	engine := ghost.StartEngine(5)
	engine.Load()

	router := mux.NewRouter()

	router.HandleFunc("/clone", handleClone)
	router.HandleFunc("/status", handleStatus)
	router.HandleFunc("/status/{id}", handleStatus)

	httpServer := http.Server{
		Addr:    serverAddress,
		Handler: router,
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		go engine.Run()
		return httpServer.ListenAndServe()
	})

	g.Go(func() error {
		<-gCtx.Done()
		engine.Save()
		return httpServer.Shutdown(context.Background())
	})

	if err := g.Wait(); err != nil {
		fmt.Printf("\nExit Reason: %s \n", err)
	}

}

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
	"time"

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
	logStr := fmt.Sprintf("Cloned http request and registered %s\n", ghostRequestStr)
	fmt.Print(logStr)
	fmt.Fprintf(w, logStr)
}

func handleStatus(w http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)
	id, ok := vars["id"]
	if !ok {
		engineStatus := ghost.GetEngine().Status()
		startStr := fmt.Sprintf("UTC Startup Time: %s\n", engineStatus.StartupTime.UTC().Format(ghost.TimeFormat))
		uptimeStr := fmt.Sprintf("Uptime Minutes: %d\n", engineStatus.UptimeMinutes)
		reqRegStr := fmt.Sprintf("Requests Registered: %d\n", engineStatus.RequestsRegisteredCount)
		reqServStr := fmt.Sprintf("Requests Served: %d\n", engineStatus.RequestsServedCount)
		reqErrStr := fmt.Sprintf("Request Errors: %d\n", engineStatus.RequestsErrorCount)
		queueStr := fmt.Sprintf("Flux Queue: %d/%d\n",
			engineStatus.QueueLength,
			engineStatus.QueueCapacity)
		pendingStr := fmt.Sprintf("Pending: %d/%d\n", engineStatus.PendingRequestsCount, engineStatus.PendingRequestsMax)
		fmt.Fprintf(w, startStr)
		fmt.Fprintf(w, uptimeStr)
		fmt.Fprintf(w, reqRegStr)
		fmt.Fprintf(w, reqServStr)
		fmt.Fprintf(w, reqErrStr)
		fmt.Fprintf(w, queueStr)
		fmt.Fprintf(w, pendingStr)
	} else {
		uuid := uuid.MustParse(id)
		fmt.Fprintf(w, ghost.GetEngine().RequestStatus(uuid))
	}

}

func main() {

	maxPending := flag.Int("max-pending", 10000, "The maximum capacity of pending requests.")
	capacity := flag.Int("max-flux", 100, "The maximum capacity of the unprocessed request queue.")
	port := flag.Int("port", 8112, "Set the port that the server will run on.")
	load := flag.Bool("load", false, "If the ghostdb file is available, load from it.")
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

	engine := ghost.NewEngine(*maxPending, *capacity, time.Second)

	if *load {
		err := engine.Load()
		if err != nil {
			fmt.Printf("Error loading state from file.\n%s", err.Error())
		}
	}

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
		engine.Halt()
		err := engine.Save()
		if err != nil {
			return err
		}
		return httpServer.Shutdown(context.Background())
	})

	if err := g.Wait(); err != nil {
		fmt.Printf("\nExit Reason: %s \n", err)
	}

}

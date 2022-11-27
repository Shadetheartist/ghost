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
		queueStr := fmt.Sprintf("Queue: %d/%d\n",
			engineStatus.QueueLength,
			engineStatus.QueueCapacity)
		activeStr := fmt.Sprintf("Active: %d/%d\n",
			engineStatus.ActiveRequestsCount,
			engineStatus.ActiveRequestsCapacity)
		fmt.Fprintf(w, startStr)
		fmt.Fprintf(w, uptimeStr)
		fmt.Fprintf(w, reqRegStr)
		fmt.Fprintf(w, reqServStr)
		fmt.Fprintf(w, reqErrStr)
		fmt.Fprintf(w, queueStr)
		fmt.Fprintf(w, activeStr)
	} else {
		uuid := uuid.MustParse(id)

		request, exists := ghost.GetEngine().GetPendingRequest(uuid)

		if exists {
			str := fmt.Sprintf("UUID: %s\n", request.Uuid.String()) +
				fmt.Sprintf("Method: %s\n", request.Method) +
				fmt.Sprintf("URL: %s\n", request.Url) +
				fmt.Sprintf("In queue since: %s\n", request.CreatedAt.UTC().Format(ghost.TimeFormat)) +
				fmt.Sprintf("Will execute at: %s\n", request.ExecuteAt.UTC().Format(ghost.TimeFormat))
			fmt.Fprintf(w, str)

		} else {
			fmt.Fprintf(w, "Not Found\n")
		}
	}
}

func main() {

	capacity := flag.Int("capacity", 1024, "The maximum capacity of the unprocessed request queue.")
	active := flag.Int("active", 16, "The maximum capacity of active requests at any given moment.")
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

	engine := ghost.NewEngine(*capacity, *active, time.Second)

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

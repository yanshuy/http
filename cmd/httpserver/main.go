package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/yanshuy/http/internal/request"
	"github.com/yanshuy/http/internal/response"
	"github.com/yanshuy/http/internal/server"
)

func HandleRequest(w *response.Writer, r *request.Request) error {
	switch r.RequestLine.Target {
	case "/yourproblem":
		w.WriteStatus(http.StatusBadRequest)
		msg := []byte("Your problem is not my problem\n")
		w.Write(msg)

	case "/myproblem":
		w.WriteStatus(http.StatusInternalServerError)
		msg := []byte("Woopsie, my bad\n")
		w.Write(msg)

	default:
		if strings.HasPrefix(r.RequestLine.Target, "/httpbin") {
			prefixLen := len("/httpbin")
			resp, _ := http.Get("https://httpbin.org" + r.RequestLine.Target[prefixLen:])
			b := make([]byte, 32)
			for {
				n, err := resp.Body.Read(b)
				w.Write(b[:n])
				if err != nil {
					break
				}
			}
		} else {
			w.Write([]byte("All good, frfr\n"))
		}
	}
	return nil
}

func main() {
	server, err := server.Serve(":42069", HandleRequest)
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
	defer server.Close()

	log.Println("Server started on port", "42069")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Server gracefully stopped")
}

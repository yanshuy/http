package server

import (
	"log"
	"net"
	"net/http"

	"github.com/yanshuy/http-server/internal/request"
	"github.com/yanshuy/http-server/internal/response"
)

type serverState int

const (
	listening serverState = iota
	closed
)

type Server struct {
	listener net.Listener
	Handler
	state serverState
}

type Handler func(w *response.Writer, r *request.Request) error

func Serve(addr string, handler Handler) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	server := &Server{
		listener: ln,
		Handler:  handler,
		state:    listening,
	}

	go server.listen()

	return server, nil
}

func (s *Server) listen() {
	for {
		conn, err := s.listener.Accept()
		if s.state == closed {
			return
		}
		if err != nil {
			log.Println("error accepting connection", err)
			continue
		}
		log.Println("connection accepted from", conn.RemoteAddr())
		go s.handleConnection(conn)
	}
}

// Calling Close transitions the server into a closed state
func (s *Server) Close() error {
	s.state = closed
	err := s.listener.Close()
	return err
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	respWriter := response.NewResponseWriter(conn)
	request, err := request.RequestFromReader(conn)
	if err != nil {
		log.Println("Request", err)
		respWriter.WriteStatus(http.StatusBadRequest)
		return
	}

	err = s.Handler(respWriter, request)
	if err != nil {
		log.Println("handler", err)
		respWriter.WriteStatus(http.StatusInternalServerError)
		return
	}

	err = respWriter.Finish()
	if err != nil {
		log.Println(err)
	}
}

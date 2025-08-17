package server

import (
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/yanshuy/http/internal/request"
	"github.com/yanshuy/http/internal/response"
)

type Server struct {
	listener net.Listener
	Handler
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
	}

	go server.listen()

	return server, nil
}

func (s *Server) listen() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Println("accept error:", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		log.Println("connection accepted from", conn.RemoteAddr())
		go s.handleConnection(conn)
	}
}

func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	const readTimeout = 5 * time.Second
	conn.SetReadDeadline(time.Now().Add(readTimeout))

	respWriter := response.NewResponseWriter(conn)
	req, err := request.RequestFromReader(conn)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			respWriter.WriteStatus(http.StatusRequestTimeout)
			respWriter.Finish()
			return
		}
		log.Println("request: ", err)
		respWriter.WriteStatus(http.StatusBadRequest)
		respWriter.Finish()
		return
	}

	if err := s.Handler(respWriter, req); err != nil {
		log.Println("handler:", err)
		respWriter.WriteStatus(http.StatusInternalServerError)
	}

	if err := respWriter.Finish(); err != nil {
		log.Println("error writing response: ", err)
	}
}

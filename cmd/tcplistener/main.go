package main

import (
	"fmt"
	"log"
	"net"

	"github.com/yanshuy/http/internal/request"
)

func main() {
	ln, err := net.Listen("tcp", ":42069")
	if err != nil {
		log.Fatal("error listening:", err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal("error accepting:", err)
		}
		log.Println("connection accept from", conn.RemoteAddr())

		request, err := request.RequestFromReader(conn)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Request line:")
		fmt.Println("- Method: " + request.Method)
		fmt.Println("- Target: " + request.Target)
		fmt.Println("- Version: " + request.HttpVersion)
		fmt.Println("Headers:")
		for key := range request.Headers {
			fmt.Printf("- %s: %s\n", key, request.Headers.GetTest(key))
		}
		fmt.Println("Body:")
		fmt.Println(string(request.Body))
	}
}

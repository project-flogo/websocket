package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var (
	server = flag.Bool("server", false, "run the echo websocket server")
)

func main() {
	flag.Parse()
	if *server {
		startServer()
		return
	}
}

// startServer starts echo websocket server on localhost:8080/ws
func startServer() {
	middleware := http.NewServeMux()
	middleware.HandleFunc("/ws", wsHandler)
	server := http.Server{
		Addr:    "localhost:8080",
		Handler: middleware,
	}
	fmt.Println("Starting server with echo websocket service at ws://localhost:8080")
	log.Fatal(server.ListenAndServe())
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("received incomming request")

	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("upgrade error", err)
	} else {
		defer conn.Close()
		//upgraded to websocket connection
		clientAdd := conn.RemoteAddr()
		fmt.Println("Upgraded to websocket protocol")
		fmt.Println("Remote address:", clientAdd)

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("read error", err)
				break
			}
			messageToLog := fmt.Sprintf("Received message(%s) from the client(%s)", message, clientAdd)
			fmt.Println(messageToLog)
		}
		return
	}
}

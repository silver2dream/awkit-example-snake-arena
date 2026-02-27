package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"awkit-example-snake-arena-backend/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := port
	if port[0] != ':' {
		addr = ":" + port
	}

	roomManager := server.NewRoomManager(120 * time.Millisecond)
	httpServer := server.NewHTTPServer(roomManager)

	log.Printf("snake arena backend listening on %s", addr)
	if err := http.ListenAndServe(addr, httpServer.Handler()); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}

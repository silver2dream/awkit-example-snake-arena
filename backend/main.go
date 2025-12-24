package main

import (
	"log"
	"net/http"
	"os"

	"awkit-example-snake-arena-backend/internal/modules/game"
)

func main() {
	manager := game.NewRoomManager(game.RoomConfig{})
	mux := http.NewServeMux()
	mux.Handle("/ws/rooms/", manager)

	addr := ":" + port()
	log.Printf("starting websocket server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func port() string {
	p := os.Getenv("PORT")
	if p == "" {
		return "8080"
	}
	return p
}

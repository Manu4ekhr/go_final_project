package main

import (
	"log"
	"net/http"
	"os"

	"go_final_project/internal/app"
)

func main() {
	port := os.Getenv("TODO_PORT")
	if port == "" {
		port = "7540"
	}

	// Инициализируем сервер
	server := app.NewServer()

	// Запускаем сервер
	log.Printf("Сервер запущен на http://localhost:%s/", port)
	log.Fatal(http.ListenAndServe(":"+port, server.Router))
}

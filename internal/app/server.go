package app

import (
	"context"
	"go_final_project/internal/handlers"
	"go_final_project/internal/storage"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Структура сервера
type Server struct {
	Router *http.ServeMux
	DB     *storage.DB
}

// NewServer создаёт сервер и подключает БД
func NewServer() *Server {
	db, err := storage.NewDB()
	if err != nil {
		log.Fatal("Ошибка при подключении к БД:", err)
	}

	server := &Server{
		Router: http.NewServeMux(),
		DB:     db,
	}

	// Раздаём файлы фронтенда
	webDir := "./web"
	fs := http.FileServer(http.Dir(webDir))
	server.Router.Handle("/", fs)

	// Подключаем маршруты
	server.setupRoutes()

	return server
}

// setupRoutes - регистрация маршрутов
func (s *Server) setupRoutes() {
	s.Router.HandleFunc("/api/nextdate", handlers.NextDateHandler)
	s.Router.HandleFunc("/api/task", handlers.TaskHandler(s.DB))
	s.Router.HandleFunc("/api/tasks", handlers.TasksHandler(s.DB))
	s.Router.HandleFunc("/api/task/done", handlers.DoneTaskHandler(s.DB))
}

// Start - запуск сервера с обработкой завершения
func (s *Server) Start(addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s.Router,
	}

	// Канал для обработки завершения
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Сервер запущен на", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка запуска сервера: %v", err)
		}
	}()

	// Ожидание сигнала завершения
	<-shutdownSignal
	log.Println("Выключение сервера...")

	// Контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка завершения сервера: %v", err)
	}

	// Закрываем БД
	s.DB.Close()
	log.Println("Сервер остановлен.")

	return nil
}

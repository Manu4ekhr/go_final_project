package handlers

import (
	"encoding/json"
	"go_final_project/internal/storage"
	"net/http"
	"strings"
	"time"
)

// TaskRequest - структура для обработки входных данных
type TaskRequest struct {
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment,omitempty"`
	Repeat  string `json:"repeat"`
}

func TaskHandler(db *storage.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			GetTaskHandler(db)(w, r)
		case http.MethodPost:
			AddTaskHandler(db)(w, r)
		case http.MethodPut:
			UpdateTaskHandler(db)(w, r)
		case http.MethodDelete:
			DeleteTaskHandler(db)(w, r)
		default:
			http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		}
	}
}

// AddTaskHandler - обработчик для добавления новой задачи
func AddTaskHandler(db *storage.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")

		var req TaskRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, `{"error": "Ошибка парсинга JSON"}`, http.StatusBadRequest)
			return
		}
		if req.Title == "" {
			http.Error(w, `{"error": "Не указан заголовок задачи"}`, http.StatusBadRequest)
			return
		}

		now := time.Now()

		if req.Date == "" {
			req.Date = now.Format("20060102")
		}

		taskDate, err := time.Parse("20060102", req.Date)
		if err != nil {
			http.Error(w, `{"error": "Некорректный формат даты"}`, http.StatusBadRequest)
			return
		}

		if taskDate.Format("20060102") < now.Format("20060102") {
			if req.Repeat == "" {
				req.Date = now.Format("20060102")
			} else {
				newDate, err := storage.NextDate(now, req.Date, req.Repeat)
				if err != nil {
					http.Error(w, `{"error": "Некорректное правило повторения"}`, http.StatusBadRequest)
					return
				}
				req.Date = newDate
			}
		}

		task := storage.Task{
			Date:    req.Date,
			Title:   req.Title,
			Comment: req.Comment,
			Repeat:  req.Repeat,
		}

		id, err := db.AddTask(task)
		if err != nil {
			http.Error(w, `{"error": "Ошибка сохранения задачи"}`, http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]int64{"id": id})
	}
}

// GetTaskHandler - обработчик получения задачи по ID
func GetTaskHandler(db *storage.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		taskID := r.URL.Query().Get("id")
		if taskID == "" {
			http.Error(w, `{"error": "Не указан идентификатор"}`, http.StatusBadRequest)
			return
		}
		task, err := db.GetTaskByID(taskID)
		if err != nil {
			http.Error(w, `{"error": "Задача не найдена"}`, http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(task)
	}
}

// TasksHandler - обработчик для получения списка задач
func TasksHandler(db *storage.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")

		searchQuery := r.URL.Query().Get("search")
		tasks, err := db.GetTasks(searchQuery)
		if err != nil {
			http.Error(w, `{"error": "Ошибка получения задач"}`, http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
	}
}

// UpdateTaskHandler - обработчик обновления задачи
func UpdateTaskHandler(db *storage.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var task storage.Task
		err := json.NewDecoder(r.Body).Decode(&task)
		if err != nil {
			http.Error(w, `{"error": "Ошибка парсинга JSON"}`, http.StatusBadRequest)
			return
		}
		if task.ID == "0" {
			http.Error(w, `{"error": "Не указан ID задачи"}`, http.StatusBadRequest)
			return
		}
		if _, err := time.Parse("20060102", task.Date); err != nil {
			http.Error(w, `{"error": "Некорректная дата"}`, http.StatusBadRequest)
			return
		}
		if task.Title == "" {
			http.Error(w, `{"error": "Не указан заголовок задачи"}`, http.StatusBadRequest)
			return
		}
		if task.Repeat != "" {
			if task.Repeat != "y" {
				parts := strings.Split(task.Repeat, " ")
				if len(parts) != 2 || parts[0] != "d" {
					http.Error(w, `{"error": "Некорректное правило повторения"}`, http.StatusBadRequest)
					return
				}
			}
		}
		err = db.UpdateTask(task)
		if err != nil {
			http.Error(w, `{"error": "Ошибка обновления задачи"}`, http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	}
}

// DeleteTaskHandler - обработчик удаления задачи
func DeleteTaskHandler(db *storage.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		taskID := r.URL.Query().Get("id")
		if taskID == "" {
			http.Error(w, `{"error": "Не указан идентификатор"}`, http.StatusBadRequest)
			return
		}

		_, err := db.GetTaskByID(taskID)
		if err != nil {
			http.Error(w, `{"error": "Некорректный идентификатор"}`, http.StatusBadRequest)
			return
		}

		err = db.DeleteTask(taskID)
		if err != nil {
			http.Error(w, `{"error": "Ошибка удаления задачи"}`, http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{})

	}
}

func DoneTaskHandler(db *storage.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		taskID := r.URL.Query().Get("id")
		if taskID == "" {
			http.Error(w, `{"error": "Не указан идентификатор"}`, http.StatusBadRequest)
			return
		}

		task, err := db.GetTaskByID(taskID)
		if err != nil {
			http.Error(w, `{"error": "Некорректный идентификатор"}`, http.StatusBadRequest)
			return
		}

		if task.Repeat == "" {
			err = db.DeleteTask(taskID)
			if err != nil {
				http.Error(w, `{"error": "Ошибка удаления задачи"}`, http.StatusInternalServerError)
				return
			}
		} else {
			nextDate, err := storage.NextDate(time.Now(), task.Date, task.Repeat)
			if err != nil {
				http.Error(w, `{"error": "Ошибка следующей даты"`, http.StatusInternalServerError)
				return
			}

			task.Date = nextDate
			err = db.UpdateTask(*task)
			if err != nil {
				http.Error(w, `{"error": "Ошибка обновления задачи"}`, http.StatusInternalServerError)
				return
			}
		}

		json.NewEncoder(w).Encode(map[string]string{})
	}
}

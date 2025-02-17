package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "modernc.org/sqlite" // Подключаем SQLite
)

const defaultPort = "7540"

// TaskDB структура для хранения задач из БД
type TaskDB struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment,omitempty"`
	Repeat  string `json:"repeat"`
}

// Функция для получения пути к БД (из переменной окружения или по умолчанию)
func getDBPath() string {
	dbPath := os.Getenv("TODO_DBFILE")
	if dbPath != "" {
		return dbPath
	}

	appPath, err := os.Executable()
	if err != nil {
		log.Fatal("Ошибка при получении пути к приложению:", err)
	}
	return filepath.Join(filepath.Dir(appPath), "scheduler.db")
}

// Функция для подключения к базе и её создания (если она отсутствует)
func initDB() (*sql.DB, error) {
	dbPath := getDBPath()
	_, err := os.Stat(dbPath)

	install := false
	if os.IsNotExist(err) {
		install = true
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("Ошибка при открытии БД: %v", err)
	}

	if install {
		log.Println("База данных не найдена, создаём новую...")
		_, err = db.Exec(`
			CREATE TABLE scheduler (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				date TEXT NOT NULL,
				title TEXT NOT NULL,
				comment TEXT,
				repeat TEXT(128)
			);
			CREATE INDEX idx_scheduler_date ON scheduler(date);
		`)
		if err != nil {
			return nil, fmt.Errorf("Ошибка при создании таблицы: %v", err)
		}
		log.Println("База данных успешно создана!")
	}

	return db, nil
}

// Обработчик API /api/nextdate
func nextDateHandler(w http.ResponseWriter, r *http.Request) {
	nowStr := r.FormValue("now")
	dateStr := r.FormValue("date")
	repeatStr := r.FormValue("repeat")

	now, err := time.Parse("20060102", nowStr)
	if err != nil {
		http.Error(w, "Некорректный формат now", http.StatusBadRequest)
		return
	}

	nextDate, err := NextDate(now, dateStr, repeatStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Fprintln(w, nextDate)
}

// Обработчик для добавления задачи (POST /api/task)
func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем, что метод POST
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Декодируем JSON-запрос
	var task Task
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		http.Error(w, `{"error": "Ошибка парсинга JSON"}`, http.StatusBadRequest)
		return
	}

	// Проверяем, указан ли заголовок задачи
	if task.Title == "" {
		http.Error(w, `{"error": "Не указан заголовок задачи"}`, http.StatusBadRequest)
		return
	}

	// Если дата пустая, берём сегодняшнюю
	if task.Date == "" {
		task.Date = time.Now().Format("20060102")
	} else {
		// Проверяем, корректный ли формат даты
		_, err := time.Parse("20060102", task.Date)
		if err != nil {
			http.Error(w, `{"error": "Некорректный формат даты"}`, http.StatusBadRequest)
			return
		}
	}

	// Если дата меньше сегодняшней и есть правило повторения → вычисляем следующую дату
	today := time.Now()
	taskDate, _ := time.Parse("20060102", task.Date)
	if taskDate.Before(today) && task.Repeat != "" {
		nextDate, err := NextDate(today, task.Date, task.Repeat)
		if err != nil {
			http.Error(w, `{"error": "Ошибка в правиле повторения"}`, http.StatusBadRequest)
			return
		}
		task.Date = nextDate
	}

	// Сохраняем задачу в базу
	id, err := saveTask(task)
	if err != nil {
		http.Error(w, `{"error": "Ошибка сохранения задачи"}`, http.StatusInternalServerError)
		return
	}

	// Отправляем ответ в JSON
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

// Функция для сохранения задачи в базу данных
func saveTask(task Task) (int64, error) {
	db, err := initDB()
	if err != nil {
		return 0, err
	}
	defer db.Close()

	query := `INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)`
	res, err := db.Exec(query, task.Date, task.Title, task.Comment, task.Repeat)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

// Обработчик для получения списка задач (GET /api/tasks)
func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	// Открываем базу данных
	db, err := initDB()
	if err != nil {
		http.Error(w, `{"error": "Ошибка подключения к БД"}`, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Выполняем SQL-запрос
	rows, err := db.Query("SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date ASC LIMIT 20")
	if err != nil {
		http.Error(w, `{"error": "Ошибка выполнения запроса"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Читаем результаты запроса
	var tasks []TaskDB
	for rows.Next() {
		var task TaskDB
		err := rows.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
		if err != nil {
			http.Error(w, `{"error": "Ошибка обработки данных"}`, http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, task)
	}

	// Отправляем JSON-ответ
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(map[string][]TaskDB{"tasks": tasks})
}

// Обработчик для получения задачи по ID (GET /api/task?id=<идентификатор>)
func getTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем ID задачи из параметров запроса
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, `{"error": "Не указан идентификатор"}`, http.StatusBadRequest)
		return
	}

	// Подключаемся к базе данных
	db, err := initDB()
	if err != nil {
		http.Error(w, `{"error": "Ошибка подключения к БД"}`, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Выполняем SQL-запрос для получения задачи
	query := "SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?"
	row := db.QueryRow(query, taskID)

	var task TaskDB
	err = row.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error": "Задача не найдена"}`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `{"error": "Ошибка выполнения запроса"}`, http.StatusInternalServerError)
		return
	}

	// Возвращаем JSON с данными задачи
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	json.NewEncoder(w).Encode(task)
}

// Обработчик для обновления задачи (PUT /api/task)
func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем, что метод PUT
	if r.Method != http.MethodPut {
		http.Error(w, `{"error": "Метод не поддерживается"}`, http.StatusMethodNotAllowed)
		return
	}

	// Декодируем JSON-запрос
	var task TaskDB
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		http.Error(w, `{"error": "Ошибка парсинга JSON"}`, http.StatusBadRequest)
		return
	}

	// Проверяем, указан ли ID задачи
	if task.ID == "" {
		http.Error(w, `{"error": "Не указан идентификатор задачи"}`, http.StatusBadRequest)
		return
	}

	// Проверяем, указан ли заголовок задачи
	if task.Title == "" {
		http.Error(w, `{"error": "Не указан заголовок задачи"}`, http.StatusBadRequest)
		return
	}

	// Проверяем формат даты
	if task.Date != "" {
		_, err := time.Parse("20060102", task.Date)
		if err != nil {
			http.Error(w, `{"error": "Некорректный формат даты"}`, http.StatusBadRequest)
			return
		}
	}

	// Подключаемся к базе данных
	db, err := initDB()
	if err != nil {
		http.Error(w, `{"error": "Ошибка подключения к БД"}`, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Обновляем задачу в базе данных
	query := `UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat = ? WHERE id = ?`
	res, err := db.Exec(query, task.Date, task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		http.Error(w, `{"error": "Ошибка обновления задачи"}`, http.StatusInternalServerError)
		return
	}

	// Проверяем, была ли обновлена задача
	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		http.Error(w, `{"error": "Задача не найдена"}`, http.StatusNotFound)
		return
	}

	// Возвращаем пустой JSON при успешном обновлении
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write([]byte("{}"))
}

// Обработчик для выполнения задачи (POST /api/task/done)
func taskDoneHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем ID задачи из параметров запроса
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, `{"error": "Не указан идентификатор задачи"}`, http.StatusBadRequest)
		return
	}

	// Подключаемся к базе данных
	db, err := initDB()
	if err != nil {
		http.Error(w, `{"error": "Ошибка подключения к БД"}`, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Получаем текущую задачу
	var task TaskDB
	query := "SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?"
	err = db.QueryRow(query, taskID).Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error": "Задача не найдена"}`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `{"error": "Ошибка выполнения запроса"}`, http.StatusInternalServerError)
		return
	}

	// Если задача одноразовая, удаляем её
	if task.Repeat == "" {
		_, err = db.Exec("DELETE FROM scheduler WHERE id = ?", taskID)
		if err != nil {
			http.Error(w, `{"error": "Ошибка удаления задачи"}`, http.StatusInternalServerError)
			return
		}
	} else {
		// Если задача повторяющаяся, вычисляем следующую дату
		now := time.Now()
		nextDate, err := NextDate(now, task.Date, task.Repeat)
		if err != nil {
			http.Error(w, `{"error": "Ошибка вычисления следующей даты"}`, http.StatusBadRequest)
			return
		}

		// Обновляем дату в базе данных
		_, err = db.Exec("UPDATE scheduler SET date = ? WHERE id = ?", nextDate, taskID)
		if err != nil {
			http.Error(w, `{"error": "Ошибка обновления даты задачи"}`, http.StatusInternalServerError)
			return
		}
	}

	// Отправляем пустой JSON-ответ
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write([]byte("{}"))
}

// Обработчик для удаления задачи (DELETE /api/task)
func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем ID задачи из параметров запроса
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, `{"error": "Не указан идентификатор задачи"}`, http.StatusBadRequest)
		return
	}

	// Подключаемся к базе данных
	db, err := initDB()
	if err != nil {
		http.Error(w, `{"error": "Ошибка подключения к БД"}`, http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Удаляем задачу из базы данных
	_, err = db.Exec("DELETE FROM scheduler WHERE id = ?", taskID)
	if err != nil {
		http.Error(w, `{"error": "Ошибка удаления задачи"}`, http.StatusInternalServerError)
		return
	}

	// Отправляем пустой JSON-ответ
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write([]byte("{}"))
}

var jwtKey = []byte("my_secret_key") // Секретный ключ для подписи токена

func signinHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}

	// Декодируем JSON
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, `{"error": "Ошибка парсинга JSON"}`, http.StatusBadRequest)
		return
	}

	// Проверяем пароль
	password := os.Getenv("TODO_PASSWORD")
	if password == "" || req.Password != password {
		http.Error(w, `{"error": "Неверный пароль"}`, http.StatusUnauthorized)
		return
	}

	// Создаём токен
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": time.Now().Add(8 * time.Hour).Unix(), // Время жизни токена - 8 часов
	})

	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		http.Error(w, `{"error": "Ошибка создания токена"}`, http.StatusInternalServerError)
		return
	}

	// Возвращаем токен
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		password := os.Getenv("TODO_PASSWORD")
		if password == "" {
			next(w, r)
			return
		}

		tokenCookie, err := r.Cookie("token")
		if err != nil {
			http.Error(w, `{"error": "Аутентификация требуется"}`, http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenCookie.Value, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, `{"error": "Неверный токен"}`, http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func main() {
	port := os.Getenv("TODO_PORT")
	if port == "" {
		port = defaultPort
	}

	// Запускаем базу данных
	db, err := initDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	log.Println("База данных подключена!")

	// Подключаем обработчики API
	http.HandleFunc("/api/nextdate", nextDateHandler)
	http.HandleFunc("/api/task", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getTaskHandler(w, r) // Получение задачи по ID
		case http.MethodPost:
			addTaskHandler(w, r) // Добавление задачи
		case http.MethodPut:
			updateTaskHandler(w, r) // Редактирование задачи
		case http.MethodDelete:
			deleteTaskHandler(w, r) // Удаление задачи
		default:
			http.Error(w, `{"error": "Метод не поддерживается"}`, http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/api/task/done", taskDoneHandler) // Завершение задачи
	http.HandleFunc("/api/tasks", getTasksHandler)     // Список ближайших задач

	// Подключаем раздачу фронтенда
	webDir := "./web"
	fs := http.FileServer(http.Dir(webDir))
	http.Handle("/", fs)

	// Запускаем сервер
	log.Printf("Сервер запущен на http://localhost:%s/", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))

	http.HandleFunc("/api/task", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getTaskHandler(w, r)
		case http.MethodPost:
			addTaskHandler(w, r)
		case http.MethodPut:
			updateTaskHandler(w, r)
		case http.MethodDelete:
			deleteTaskHandler(w, r)
		default:
			http.Error(w, `{"error": "Метод не поддерживается"}`, http.StatusMethodNotAllowed)
		}
	}))

	http.HandleFunc("/api/tasks", authMiddleware(getTasksHandler))
	http.HandleFunc("/api/task/done", authMiddleware(taskDoneHandler))
	http.HandleFunc("/api/signin", signinHandler)

}

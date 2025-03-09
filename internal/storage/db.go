package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Импорт драйвера SQLite
)

// Константы
const (
	TaskLimit = 20 // Глобальная константа для лимита задач
)

// DB - структура для работы с базой данных
type DB struct {
	conn *sql.DB
}

type Task struct {
	ID      string `json:"id"`
	Date    string `json:"date"` // В формате 20060102
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

// getDBPath - получение пути к файлу базы данных
func getDBPath() string {
	dbPath := os.Getenv("TODO_DBFILE")
	if dbPath != "" {
		return dbPath
	}

	// Определяем путь к БД по умолчанию
	appPath, err := os.Executable()
	if err != nil {
		log.Fatal("Ошибка при получении пути к приложению:", err)
	}
	return filepath.Join(filepath.Dir(appPath), "scheduler.db")
}

// NewDB - функция для инициализации БД
func NewDB() (*DB, error) {
	dbFile := getDBPath()
	_, err := os.Stat(dbFile)
	install := os.IsNotExist(err) // Если файла нет, нужно создать таблицы

	// Открываем БД
	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка при открытии БД: %v", err)
	}

	// Если БД новая, создаём таблицу
	if install {
		if err := createTables(db); err != nil {
			return nil, fmt.Errorf("ошибка при создании таблицы: %v", err)
		}
		fmt.Println("База данных успешно создана!")
	}

	return &DB{conn: db}, nil
}

// createTables - создаёт таблицу и индекс в БД
func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS scheduler (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT NOT NULL,
		title TEXT NOT NULL,
		comment TEXT,
		repeat TEXT(128)
	);
	CREATE INDEX IF NOT EXISTS idx_scheduler_date ON scheduler(date);
	`
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("ошибка при создании таблицы: %v", err)
	}
	return nil
}

// AddTask - добавление новой задачи
func (db *DB) AddTask(task Task) (int64, error) {
	query := `INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?) RETURNING id`

	res, err := db.conn.Exec(query, task.Date, task.Title, task.Comment, task.Repeat)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetTasks - получение списка ближайших задач с возможностью поиска
func (db *DB) GetTasks(search string) ([]Task, error) {
	var rows *sql.Rows
	var rowsErr error

	if search != "" {
		searchDate, err := time.Parse("02.01.2006", search)
		if err == nil {
			searchFormatted := searchDate.Format(DateFormat)
			rows, rowsErr = db.conn.Query("SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date ASC LIMIT ?", searchFormatted, TaskLimit)
		} else {
			searchLike := "%" + search + "%"
			rows, rowsErr = db.conn.Query("SELECT id, date, title, comment, repeat FROM scheduler WHERE title LIKE ? OR comment LIKE ? ORDER BY date ASC LIMIT ?", searchLike, searchLike, TaskLimit)
		}
	} else {
		rows, rowsErr = db.conn.Query("SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date ASC LIMIT ?", TaskLimit)
	}

	if rowsErr != nil {
		return nil, rowsErr
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	// Проверяем наличие ошибки после итерации по строкам
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке результатов запроса: %v", err)
	}

	return tasks, nil
}

// GetTaskByID - получение задачи по ID
func (db *DB) GetTaskByID(taskID string) (*Task, error) {
	var task Task
	err := db.conn.QueryRow("SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?", taskID).
		Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("задача не найдена")
	} else if err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTask - обновление задачи в БД
func (db *DB) UpdateTask(task Task) error {
	query := `UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat = ? WHERE id = ?`
	res, err := db.conn.Exec(query, task.Date, task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		return fmt.Errorf("ошибка обновления задачи: %v", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return fmt.Errorf("задача не найдена")
	}
	return nil
}

// DeleteTask - удаление задачи
func (db *DB) DeleteTask(taskID string) error {
	_, err := db.conn.Exec("DELETE FROM scheduler WHERE id = ?", taskID)
	return err
}

// Close - закрывает соединение с БД
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

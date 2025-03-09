package handlers

import (
	"go_final_project/internal/storage"
	"net/http"
	"time"
)

// NextDateHandler - обработчик для API "/api/nextdate"
func NextDateHandler(w http.ResponseWriter, r *http.Request) {
	nowStr := r.FormValue("now")
	date := r.FormValue("date")
	repeat := r.FormValue("repeat")

	if nowStr == "" || date == "" || repeat == "" {
		http.Error(w, `{"error": "Не все параметры переданы"}`, http.StatusBadRequest)
		return
	}

	now, err := time.Parse("20060102", nowStr)
	if err != nil {
		http.Error(w, `{"error": "Некорректная текущая дата"}`, http.StatusBadRequest)
		return
	}

	nextDate, err := storage.NextDate(now, date, repeat)
	if err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Write([]byte(nextDate))
}

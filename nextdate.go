package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NextDate вычисляет следующую дату выполнения задачи
func NextDate(now time.Time, date string, repeat string) (string, error) {
	startDate, err := time.Parse("20060102", date)
	if err != nil {
		return "", fmt.Errorf("некорректный формат даты: %s", date)
	}

	if repeat == "" {
		return "", fmt.Errorf("задача не повторяется")
	}

	parts := strings.Split(repeat, " ")

	switch parts[0] {
	case "d":
		if len(parts) != 2 {
			return "", fmt.Errorf("некорректный формат повторения: %s", repeat)
		}
		days, err := strconv.Atoi(parts[1])
		if err != nil || days <= 0 || days > 400 {
			return "", fmt.Errorf("некорректное количество дней: %s", parts[1])
		}

		next := startDate
		for !next.After(now) {
			next = next.AddDate(0, 0, days)
		}
		return next.Format("20060102"), nil

	case "y":
		next := startDate.AddDate(1, 0, 0)
		if !next.After(now) {
			next = next.AddDate(1, 0, 0)
		}
		return next.Format("20060102"), nil

	default:
		return "", fmt.Errorf("неподдерживаемый формат повторения: %s", repeat)
	}
}

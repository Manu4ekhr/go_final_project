package storage

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DateFormat - константа формата даты для парсинга и вывода
const DateFormat = "20060102"

// NextDate вычисляет следующую дату выполнения задачи
func NextDate(now time.Time, date string, repeat string) (string, error) {
	startDate, err := time.Parse(DateFormat, date)
	if err != nil {
		return "", fmt.Errorf("некорректный формат даты: %s", date)
	}

	if repeat == "" {
		return "", fmt.Errorf("задача не повторяется")
	}

	parts := strings.Split(repeat, " ")

	switch parts[0] {
	case "d": // Повтор каждые X дней
		if len(parts) != 2 {
			return "", fmt.Errorf("некорректный формат повторения: %s", repeat)
		}
		days, err := strconv.Atoi(parts[1])
		if err != nil || days <= 0 || days > 400 {
			return "", fmt.Errorf("некорректное количество дней: %s", parts[1])
		}

		next := startDate.AddDate(0, 0, days)
		for next.Format(DateFormat) <= now.Format(DateFormat) {
			next = next.AddDate(0, 0, days)
		}

		return next.Format(DateFormat), nil

	case "y": // Ежегодное повторение
		next := startDate.AddDate(1, 0, 0)
		for next.Format(DateFormat) <= now.Format(DateFormat) {
			next = next.AddDate(1, 0, 0)
		}

		return next.Format(DateFormat), nil

	default:
		return "", fmt.Errorf("неподдерживаемый формат повторения: %s", repeat)
	}
}

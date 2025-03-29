package rss

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dromara/carbon/v2"
)

var numberReg = regexp.MustCompile(`\d+`)

// FilterCronTasks filters cron tasks based on the current date and task type
func FilterCronTasks(tasks []CronTask) []CronTaskRes {
	now := carbon.Now()
	var filteredTasks []CronTaskRes

	for _, task := range tasks {
		if shouldShowTask(task.Type, now) {
			for _, s := range task.Item {
				filteredTasks = append(filteredTasks, CronTaskRes{
					Type: fmt.Sprintf("@%s", task.Type),
					Task: s.Task,
				})
			}
		}
	}

	return filteredTasks
}

// shouldShowTask determines if a task should be shown based on its type and current date
func shouldShowTask(taskType string, now carbon.Carbon) bool {
	// Handle numeric prefixes (e.g., "2daily", "4weekly")
	isMatched, number := extractTimeNumber(taskType)
	baseType := strings.TrimLeftFunc(taskType, func(r rune) bool {
		return r >= '0' && r <= '9'
	})

	switch baseType {
	case "daily":
		return true // Daily tasks are always shown
	case "weekly":
		return now.IsSaturday() && (!isMatched || now.WeekOfYear()%number == 0)
	case "monthly":
		return now.Day() == 1 && (!isMatched || now.Month()%number == 0)
	case "yearly":
		return now.IsJanuary() && now.Day() == 1
	default:
		return false
	}
}

// extractTimeNumber extracts the numeric value from a cron type string
func extractTimeNumber(t string) (isMatched bool, number int) {
	isMatched = numberReg.MatchString(t)
	if !isMatched {
		return isMatched, 1
	}
	number, _ = strconv.Atoi(numberReg.FindString(t))
	return
}

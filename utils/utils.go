package utils

import (
	"fmt"
	"github.com/viant/toolbox"
	"strings"
	"time"
)

func StringToTime(ts string) *time.Time {
	layout := "2006-01-02 15:04:05.000"
	if strings.Contains(ts, "T") {
		layout = time.RFC3339Nano
	} else {
		layout = "2006-01-02 15:04:05.000-07"
	}
	t, err := toolbox.ToTime(ts, layout)
	if err != nil {
		fmt.Println("failed to convert time: %s %v\n", ts, err)
		return nil
	}
	return t
}

func TimeMsToString(millis int64) string {
	layout := "2006-01-02 15:04:05.000-07"
	time.Unix(0, millis*int64(time.Millisecond))
	return time.Unix(0, millis*int64(time.Millisecond)).Format(layout)
}

func StringToDate(ts string) int32 {
	layout := "2006-01-02"
	if strings.Contains(ts, "T") {
		layout = time.RFC3339Nano
	} else {
		layout = "2006-01-02 15:04:05.000-07"
	}

	t, err := toolbox.ToTime(ts, layout)
	if err != nil {
		fmt.Println("failed to convert time: %s %v\n", ts, err)
		return 0
	}
	daysInSec := t.Unix() + 1
	days := daysInSec / 3600 / 24
	if days*3600*24 < daysInSec {
		days += 1
	}
	return int32(days)
}

func DateToString(date int32) string {
	t := time.Unix(int64(date*3600*24), 0)
	return t.Format("2006-01-02")
}
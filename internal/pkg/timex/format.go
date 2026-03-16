// Package timeutil 提供统一的时间格式化工具
package timex

import "time"

// 预定义的时间格式常量
const (
	// DateFormat 日期格式 (YYYY-MM-DD)
	// 用于日期比较、日志记录等场景
	DateFormat = "2006-01-02"

	// DateTimeFormat 日期时间格式 (YYYY-MM-DD HH:MM:SS)
	// 用于完整的时间戳显示
	DateTimeFormat = "2006-01-02 15:04:05"

	// ISO8601Format ISO 8601 格式 (YYYY-MM-DDTHH:MM:SS)
	// 用于API交互、数据交换
	ISO8601Format = "2006-01-02T15:04:05"

	// FileTimestampFormat 文件名时间戳格式 (YYYYMMDD_HHMMSS)
	// 用于生成带时间戳的文件名
	FileTimestampFormat = "20060102_150405"

	// CompactDateFormat 紧凑日期格式 (YYYYMMDD)
	// 用于AWS签名、紧凑的日期表示
	CompactDateFormat = "20060102"

	// CompactDateTimeFormat 紧凑日期时间格式 (YYYYMMDDTHHMMSSZ)
	// 用于AWS签名、API请求头
	CompactDateTimeFormat = "20060102T150405Z"

	// LogTimestampFormat 日志时间戳格式 (YYYYMMDD-HHMMSS)
	// 用于日志文件轮转
	LogTimestampFormat = "20060102-150405"
)

// FormatDate 格式化为日期 (YYYY-MM-DD)
func FormatDate(t time.Time) string {
	return t.Format(DateFormat)
}

// FormatDateTime 格式化为日期时间 (YYYY-MM-DD HH:MM:SS)
func FormatDateTime(t time.Time) string {
	return t.Format(DateTimeFormat)
}

// FormatISO8601 格式化为ISO 8601格式 (YYYY-MM-DDTHH:MM:SS)
func FormatISO8601(t time.Time) string {
	return t.Format(ISO8601Format)
}

// FormatFileTimestamp 格式化为文件名时间戳 (YYYYMMDD_HHMMSS)
func FormatFileTimestamp(t time.Time) string {
	return t.Format(FileTimestampFormat)
}

// FormatCompactDate 格式化为紧凑日期 (YYYYMMDD)
func FormatCompactDate(t time.Time) string {
	return t.Format(CompactDateFormat)
}

// FormatCompactDateTime 格式化为紧凑日期时间 (YYYYMMDDTHHMMSSZ)
func FormatCompactDateTime(t time.Time) string {
	return t.Format(CompactDateTimeFormat)
}

// FormatLogTimestamp 格式化为日志时间戳 (YYYYMMDD-HHMMSS)
func FormatLogTimestamp(t time.Time) string {
	return t.Format(LogTimestampFormat)
}

// NowDate 获取当前日期字符串 (YYYY-MM-DD)
func NowDate() string {
	return FormatDate(time.Now())
}

// NowDateTime 获取当前日期时间字符串 (YYYY-MM-DD HH:MM:SS)
func NowDateTime() string {
	return FormatDateTime(time.Now())
}

// NowFileTimestamp 获取当前文件名时间戳 (YYYYMMDD_HHMMSS)
func NowFileTimestamp() string {
	return FormatFileTimestamp(time.Now())
}

// IsSameDate 判断两个时间是否为同一天
func IsSameDate(t1, t2 time.Time) bool {
	return FormatDate(t1) == FormatDate(t2)
}

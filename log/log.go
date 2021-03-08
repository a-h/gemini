package log

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Field func() (k string, v interface{})

func String(name string, value string) Field {
	return func() (k string, v interface{}) {
		return name, value
	}
}
func Int(name string, value int) Field {
	return func() (k string, v interface{}) {
		return name, value
	}
}
func Int64(name string, value int64) Field {
	return func() (k string, v interface{}) {
		return name, value
	}
}
func Interface(name string, value interface{}) Field {
	return func() (k string, v interface{}) {
		return name, value
	}
}

func Info(msg string, fields ...Field) {
	std.Write("INFO", msg, fields...)
}

func Warn(msg string, fields ...Field) {
	std.Write("WARN", msg, fields...)
}

func Error(msg string, err error, fields ...Field) {
	if err != nil {
		fields = append(fields, String("error", err.Error()))
	}
	std.Write("ERROR", msg, fields...)
}

type Logger struct {
	stdout  sync.Mutex
	stderr  sync.Mutex
	enc     json.Encoder
	newLine []byte
}

func (s *Logger) Write(level string, msg string, fields ...Field) {
	entry := make(map[string]interface{})
	entry["time"] = time.Now().UTC().Format(time.RFC3339)
	entry["level"] = level
	entry["msg"] = msg
	for i := 0; i < len(fields); i++ {
		k, v := fields[i]()
		entry[k] = v
	}
	j, err := json.Marshal(entry)
	if err != nil {
		j = []byte(fmt.Sprintf("log.Write: failed to marshal entry '%+v': %v", entry, err))
	}
	if level == "ERROR" {
		s.stderr.Lock()
		defer s.stderr.Unlock()
		os.Stderr.Write(j)
		os.Stderr.Write(s.newLine)
		return
	}
	s.stdout.Lock()
	defer s.stdout.Unlock()
	os.Stdout.Write(j)
	os.Stderr.Write(s.newLine)
}

var std = &Logger{newLine: []byte("\n")}

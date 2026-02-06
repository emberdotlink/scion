/*
Copyright 2025 The Scion Authors.
*/

package log

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"
)

var (
	logPath  string
	debug    bool
	mu       sync.Mutex
	initialized bool
)

// Init initializes the logging system.
func Init() {
	mu.Lock()
	defer mu.Unlock()

	if initialized && logPath != "" {
		return
	}

	if logPath == "" {
		home := os.Getenv("HOME")
		if home == "" {
			home = "/home/scion"
		}
		logPath = filepath.Join(home, "agent.log")
	}

	if os.Getenv("SCION_DEBUG") != "" {
		debug = true
	}
	initialized = true
}

// SetLogPath sets the path to the log file. Primarily for testing.
func SetLogPath(path string) {
	mu.Lock()
	defer mu.Unlock()
	logPath = path
	initialized = true // Consider it initialized if path is explicitly set
}

// Info logs an informational message.
func Info(format string, args ...interface{}) {
	write("INFO", "", format, args...)
}

// TaggedInfo logs an informational message with an additional tag.
func TaggedInfo(tag string, format string, args ...interface{}) {
	write("INFO", tag, format, args...)
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	write("ERROR", "", format, args...)
}

// Debug logs a debug message if SCION_DEBUG is set.
func Debug(format string, args ...interface{}) {
	if !debug {
		return
	}
	write("DEBUG", "", format, args...)
}

func write(level, tag, format string, args ...interface{}) {
	if !initialized {
		Init()
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)

	tagStr := ""
	if tag != "" {
		tagStr = fmt.Sprintf(" [%s]", tag)
	}

	// Format for agent.log: timestamp [sciontool] [LEVEL] [TAG] message
	fileEntry := fmt.Sprintf("%s [sciontool] [%s]%s %s\n", timestamp, level, tagStr, message)

	// Format for stderr: [sciontool] LEVEL: [TAG] message
	stderrEntry := fmt.Sprintf("[sciontool] %s:%s %s\n", level, tagStr, message)

	// Write to stderr
	fmt.Fprint(os.Stderr, stderrEntry)

	// Write to agent.log
	mu.Lock()
	defer mu.Unlock()
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// If we can't write to agent.log, try to fall back to /tmp and enable debug
		if logPath != "/tmp/agent.log" {
			debug = true
			oldPath := logPath
			logPath = "/tmp/agent.log"

			// Get system info for debugging
			uid := os.Getuid()
			gid := os.Getgid()
			username := "unknown"
			if u, err := user.Current(); err == nil {
				username = u.Username
			}
			sysInfo := fmt.Sprintf("UID=%d, GID=%d, USER=%s, HOME=%s, SCION_HOST_UID=%s, SCION_HOST_GID=%s",
				uid, gid, username, os.Getenv("HOME"), os.Getenv("SCION_HOST_UID"), os.Getenv("SCION_HOST_GID"))

			fallbackMsg := fmt.Sprintf("[sciontool] WARNING: Failed to write to %s: %v. Falling back to /tmp/agent.log and enabling debug mode. %s\n", oldPath, err, sysInfo)
			fmt.Fprint(os.Stderr, fallbackMsg)

			// Retry with new path
			f, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				// Total failure
				return
			}
			// Write the fallback message to the new log file too
			f.WriteString(timestamp + " " + fallbackMsg)
		} else {
			// Already at /tmp/agent.log and it failed
			return
		}
	}
	defer f.Close()

	f.WriteString(fileEntry)
}

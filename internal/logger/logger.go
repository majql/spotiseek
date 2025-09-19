package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

var (
	debugMode bool
	logger    *log.Logger
)

func init() {
	logger = log.New(os.Stdout, "", log.LstdFlags)
}

func SetDebugMode(debug bool) {
	debugMode = debug
	if debug {
		logger.SetFlags(log.LstdFlags | log.Lshortfile)
		Info("Debug mode enabled - detailed logging activated")
	} else {
		logger.SetFlags(log.LstdFlags)
	}
}

func IsDebugMode() bool {
	return debugMode
}

func Debug(format string, v ...interface{}) {
	if debugMode {
		msg := fmt.Sprintf("[DEBUG] "+format, v...)
		logger.Output(2, msg)
	}
}

func Info(format string, v ...interface{}) {
	msg := fmt.Sprintf("[INFO] "+format, v...)
	logger.Output(2, msg)
}

func Warn(format string, v ...interface{}) {
	msg := fmt.Sprintf("[WARN] "+format, v...)
	logger.Output(2, msg)
}

func Error(format string, v ...interface{}) {
	msg := fmt.Sprintf("[ERROR] "+format, v...)
	logger.Output(2, msg)
}

func Fatal(format string, v ...interface{}) {
	msg := fmt.Sprintf("[FATAL] "+format, v...)
	logger.Output(2, msg)
	os.Exit(1)
}

func Printf(format string, v ...interface{}) {
	logger.Printf(format, v...)
}

func Println(v ...interface{}) {
	logger.Println(v...)
}

func LogOperation(operation string, start time.Time, err error) {
	duration := time.Since(start)
	if err != nil {
		Error("Operation '%s' failed after %v: %v", operation, duration, err)
	} else {
		if debugMode {
			Debug("Operation '%s' completed in %v", operation, duration)
		} else {
			Info("Operation '%s' completed", operation)
		}
	}
}

func LogHTTPRequest(method, url string, statusCode int, duration time.Duration) {
	if debugMode {
		Debug("HTTP %s %s -> %d (%v)", method, url, statusCode, duration)
	} else {
		Info("HTTP %s %s -> %d", method, url, statusCode)
	}
}

func LogDockerOperation(operation, containerName string, err error) {
	if err != nil {
		Error("Docker %s failed for container '%s': %v", operation, containerName, err)
	} else {
		Info("Docker %s successful for container '%s'", operation, containerName)
	}
}

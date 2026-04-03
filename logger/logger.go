package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
)

var AppName string

var (
	LogErr    *log.Logger
	LogWarn   *log.Logger
	LogInfo   *log.Logger
	LogAlways *log.Logger
)

func init() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		log.Fatalln("FATAL ERROR: Failed to read build info! Please build the binary with module support.")
	}
	AppName = filepath.Base(bi.Path)

	LogErr = log.New(os.Stderr, "["+AppName+"] ERROR: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile|log.LUTC)
	LogWarn = log.New(os.Stdout, "["+AppName+"] WARNING: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile|log.LUTC)
	LogInfo = log.New(os.Stdout, "["+AppName+"] INFO: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile|log.LUTC)
	LogAlways = log.New(os.Stdout, "["+AppName+"] ALWAYS: ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile|log.LUTC)
}

func Typeof(v any) string {
	return fmt.Sprintf("%T", v)
}

func Explain(v any) string {
	return fmt.Sprintf("%+v", v)
}

func FindString(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

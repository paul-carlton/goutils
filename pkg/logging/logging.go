// Package logging contains logging related functions used by multiple packages
package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

const (
	// Me is the setting for the function that called MyCaller.
	Me = 3
	// MyCaller is the setting for the function that called the function calling MyCaller.
	MyCaller = 4
	// MyCallersCaller is the setting for the function that called the function that called the function calling MyCaller.
	MyCallersCaller = 5
	// MyCallersCallersCaller is the setting for the function that called the function that called the function that called the function calling MyCaller.
	MyCallersCallersCaller = 6

	// logLevelEnvVar is the environmental variable name used to set the log level, defaults to INFO if not set.
	logLevelEnvVar = "LOG_LEVEL"
	// logLevelEnvVar is the environmental variable name used to determine if source code info is included in the log output, defaults to true if not set.
	logSourceEnvVar = "LOG_SOURCE"
	// SourcePathDepthEnvVar is the environmental variable name used to determine the number of elements of the full path name is included in thes source file
	// zero means only file name, no path elements, a positive number indicates the number of path elements to include and -1 means full path.
	// If not set, no path elements are included.
	sourcePathDepthEnvVar = "SOURCE_PATH_DEPTH"

	// LevelTrace defines a tracing log level.
	LevelTrace = slog.Level(-8)
	// LevelFatal defines a fatal error log level.
	LevelFatal = slog.Level(12)
)

var (
	errNotAvailable = errors.New("caller not availalble")

	levelNames = map[slog.Leveler]string{ //nolint: gochecknoglobals
		LevelTrace: "TRACE",
		LevelFatal: "FATAL",
	}
)

func setLogLevel() slog.Level {
	if tlevel, ok := os.LookupEnv(logLevelEnvVar); ok {
		switch tlevel {
		case "TRACE":
			return LevelTrace
		case "DEBUG":
			return slog.LevelDebug
		case "INFO":
			return slog.LevelInfo
		case "WARN":
			return slog.LevelWarn
		case "ERROR":
			return slog.LevelError
		case "FATAL":
			return LevelFatal
		default:
			fmt.Printf("Invalid tracing level: %s, defaulting to INFO", tlevel)
		}
	}
	return slog.LevelInfo
}

func setSource() bool {
	if source, ok := os.LookupEnv(logSourceEnvVar); ok {
		return source == strings.ToLower("true")
	}
	return true
}

func setSourcePathDepth() int {
	if path, ok := os.LookupEnv(sourcePathDepthEnvVar); ok {
		value, err := strconv.Atoi(path)
		if err != nil {
			fmt.Printf("Invalid source code path element count: %s, defaulting to none", path)
			return 0
		}
		return value
	}
	return 0
}

func setLogLevelName(a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		level, ok := a.Value.Any().(slog.Level)
		if !ok {
			fmt.Printf("expected slog.LevelKey, invalid slog.Attr, Key: %s, Value: %s, skipping\n", a.Key, a.Value)
			return a
		}
		levelLabel, exists := levelNames[level]
		if !exists {
			levelLabel = level.String()
		}

		a.Value = slog.StringValue(levelLabel)
	}
	return a
}

func setSourceName(a slog.Attr) slog.Attr {
	if a.Key == slog.SourceKey { //nolint: nestif
		pathElements := setSourcePathDepth()
		if pathElements >= 0 {
			source, ok := a.Value.Any().(*slog.Source)
			if !ok {
				// Not sure why this path is taken on occasion but the code works!
				// fmt.Printf("expected slog.SourceKey, invalid slog.Attr, Key: %s, Value: %s, skipping\n", a.Key, a.Value)
				return a
			}
			path := strings.Split(filepath.Dir(source.File), "/")
			if len(path) < pathElements {
				pathElements = len(path)
			}
			includedPath := strings.Join(path[len(path)-pathElements:], "/")
			sep := ""
			if len(includedPath) > 0 {
				sep = "/"
			}
			source.File = fmt.Sprintf("%s%s%s", includedPath, sep, filepath.Base(source.File))
			source.Function = filepath.Base(source.Function)
			a.Value = slog.AnyValue(source)
		}
	}
	return a
}

// NewLogger returns a logger.
func NewLogger() *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     setLogLevel(),
		AddSource: setSource(),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr { //nolint: revive
			a = setLogLevelName(a)
			a = setSourceName(a)
			return a
		},
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)

	return slog.New(handler)
}

// LogJSON is used log an item in JSON format.
func LogJSON(data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err.Error()
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonData, "", "  ")

	if err != nil {
		return err.Error()
	}

	return prettyJSON.String()
}

// GetObjNamespaceName gets object namespace and name for logging.
func GetObjNamespaceName(obj k8sruntime.Object) (result []interface{}) {
	mobj, ok := (obj).(metav1.Object)
	if !ok {
		result = append(result, "namespace", "unavailable", "name", "unavailable")

		return result
	}

	result = append(result, "namespace", mobj.GetNamespace(), "name", mobj.GetName())

	return result
}

// GetObjKindNamespaceName gets object kind namespace and name for logging.
func GetObjKindNamespaceName(obj k8sruntime.Object) (result []interface{}) {
	if obj == nil {
		result = append(result, "obj", "nil")

		return result
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	result = append(result, "kind", fmt.Sprintf("%s.%s", gvk.Kind, gvk.Group))
	result = append(result, GetObjNamespaceName(obj)...)

	return result
}

// CallerInfo hold the function name and source file/line from which a call was made.
type CallerInfo struct {
	FunctionName string
	SourceFile   string
	SourceLine   int
}

// Callers returns an array of strings containing the function name, source filename and line
// number for the caller of this function and its caller moving up the stack for as many levels as
// are available or the number of levels specified by the levels parameter.
// Set the short parameter to true to only return final element of Function and source file name.
func Callers(levels uint, short bool) ([]CallerInfo, error) {
	var callers []CallerInfo

	if levels == 0 {
		return callers, nil
	}
	// We get the callers as uintptrs.
	fpcs := make([]uintptr, levels)

	// Skip 1 levels to get to the caller of whoever called Callers().
	n := runtime.Callers(1, fpcs)
	if n == 0 {
		return nil, errNotAvailable
	}

	frames := runtime.CallersFrames(fpcs)

	for {
		frame, more := frames.Next()
		if frame.Line == 0 {
			break
		}

		funcName := frame.Function
		sourceFile := frame.File
		lineNumber := frame.Line

		if short {
			funcName = filepath.Base(funcName)
			sourceFile = filepath.Base(sourceFile)
		}

		caller := CallerInfo{FunctionName: funcName, SourceFile: sourceFile, SourceLine: lineNumber}
		callers = append(callers, caller)

		if !more {
			break
		}
	}

	return callers, nil
}

// GetCaller returns the caller of GetCaller 'skip' levels back.
// Set the short parameter to true to only return final element of Function and source file name.
func GetCaller(skip uint, short bool) CallerInfo {
	callers, err := Callers(skip, short)
	if err != nil {
		return CallerInfo{FunctionName: "not available", SourceFile: "not available", SourceLine: 0}
	}

	if skip == 0 {
		return CallerInfo{FunctionName: "not available", SourceFile: "not available", SourceLine: 0}
	}

	if int(skip) > len(callers) {
		return CallerInfo{FunctionName: "not available", SourceFile: "not available", SourceLine: 0}
	}

	return callers[skip-1]
}

// CallerStr returns the caller's function, source file and line number as a string.
func CallerStr(skip uint) string {
	callerInfo := GetCaller(skip+1, true)

	return fmt.Sprintf("%s - %s(%d)", callerInfo.FunctionName, callerInfo.SourceFile, callerInfo.SourceLine)
}

// TraceCall traces calls and exit for functions.
func TraceCall(log *slog.Logger) {
	callerInfo := GetCaller(MyCaller, true)
	ctx := context.Background()
	log.Log(ctx, LevelTrace, "Entering function", "function", callerInfo.FunctionName, "source", callerInfo.SourceFile, "line", callerInfo.SourceLine)
}

// TraceExit traces calls and exit for functions.
func TraceExit(log *slog.Logger) {
	callerInfo := GetCaller(MyCaller, true)
	ctx := context.Background()
	log.Log(ctx, LevelTrace, "Exiting function", "function", callerInfo.FunctionName, "source", callerInfo.SourceFile, "line", callerInfo.SourceLine)
}

// GetFunctionAndSource gets function name and source line for logging.
func GetFunctionAndSource(skip uint) (result []interface{}) {
	callerInfo := GetCaller(skip, true)
	result = append(result, "function", callerInfo.FunctionName, "source", callerInfo.SourceFile, "line", callerInfo.SourceLine)

	return result
}

// CallerText generates a string containing caller function, source and line.
func CallerText(skip uint) string {
	callerInfo := GetCaller(skip, true)

	return fmt.Sprintf("%s(%d) %s - ", callerInfo.SourceFile, callerInfo.SourceLine, callerInfo.FunctionName)
}

// Package logging contains logging related functions used by multiple packages
package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	eleven = 11

	stackDepth = 32

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

	// levelNames defines the names of the custom logging levels.
	levelNames = map[slog.Leveler]string{ //nolint: gochecknoglobals
		LevelTrace: "TRACE",
		LevelFatal: "FATAL",
	}

	sourcePathDepth int //nolint: gochecknoglobals
	// LogLevel contains the logging level set.
	LogLevel  slog.Level //nolint: gochecknoglobals
	LogOut    io.Writer  //nolint: gochecknoglobals
	logSource bool       //nolint: gochecknoglobals

	// traceLog is used by trace logging functions that replace the source information with the callers source info.
	TraceLog *slog.Logger //nolint: gochecknoglobals
)

func init() {
	sourcePathDepth = setSourcePathDepth()
	logSource = setSource()
	LogLevel = setLogLevel()
	TraceLog = TraceLogger(os.Stderr)
}

// setLogLevel returns the logging level selected by the user.
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

// GetLogLevel gets the log level as set by environmental variable.
func GetLogLevel() string {
	return LogLevel.String()
}

// SetLogLevel sets the log level from environmental variable.
func SetLogLevel() {
	LogLevel = setLogLevel()
}

// GetLogOut gets the log output io.Writer.
func GetLogOut() io.Writer {
	return LogOut
}

// SetLogOut sets the log output io.Writer.
func SetLogOut(out io.Writer) {
	LogOut = out
}

// setSource set the user preference to include or exclude source file information in log messages.
func setSource() bool {
	if source, ok := os.LookupEnv(logSourceEnvVar); ok {
		return source == strings.ToLower("true")
	}
	return true
}

// setSourcePathDepth sets the number of path elements to include for a source file name.
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

// setLevelName sets the level string in the log message to the name of the logging level used.
// This supports custom logging levels like FATAL and TRACE as added by this package.
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

// setCallerSourceName is used to set to source information to the caller of the function calling log.
func setCallerSourceName(a slog.Attr) slog.Attr {
	if a.Key == slog.SourceKey { //nolint: nestif
		source := GetCaller(eleven, false)
		if sourcePathDepth >= 0 {
			path := strings.Split(filepath.Dir(source.File), "/")
			if len(path) < sourcePathDepth {
				sourcePathDepth = len(path)
			}
			includedPath := strings.Join(path[len(path)-sourcePathDepth:], "/")
			sep := ""
			if len(includedPath) > 0 {
				sep = "/"
			}
			source.File = fmt.Sprintf("%s%s%s", includedPath, sep, filepath.Base(source.File))
			source.Function = filepath.Base(source.Function)
		}
		a.Value = slog.AnyValue(source)
	}
	return a
}

// setSourceName is used to set to source file name, specifically the number of elements of the directory path to include.
func setSourceName(a slog.Attr) slog.Attr {
	if a.Key == slog.SourceKey { //nolint: nestif
		if sourcePathDepth >= 0 {
			source, ok := a.Value.Any().(*slog.Source)
			if !ok {
				// Not sure why this path is taken on occasion but the code works!
				// fmt.Printf("expected slog.SourceKey, invalid slog.Attr, Key: %s, Value: %s, skipping\n", a.Key, a.Value)
				return a
			}
			path := strings.Split(filepath.Dir(source.File), "/")
			if len(path) < sourcePathDepth {
				sourcePathDepth = len(path)
			}
			includedPath := strings.Join(path[len(path)-sourcePathDepth:], "/")
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

// NewLogger returns a JSON logger writing to provided writer.
func NewLoggerTo(out io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(out, setupOptions()))
}

func setupOptions() *slog.HandlerOptions {
	return &slog.HandlerOptions{
		Level:     LogLevel,
		AddSource: logSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr { //nolint: revive
			a = setLogLevelName(a)
			a = setSourceName(a)
			return a
		},
	}
}

// NewLogger returns a JSON logger.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, setupOptions()))
}

// traceLogger returns a logger for internal use by tracing that replaces the source details with supplied values.
func TraceLogger(out io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     LogLevel,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr { //nolint: revive
			a = setLogLevelName(a)
			a = setCallerSourceName(a)
			return a
		},
	}

	handler := slog.NewJSONHandler(out, opts)
	return slog.New(handler)
}

// NewTextLogger returns a text logger.
func NewTextLoggerTo(out io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: LogLevel,
	}

	handler := slog.NewTextHandler(out, opts)

	return slog.New(handler)
}

// NewTextLoggerTo returns a text logger logging to provided output.
func NewTextLogger() *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: LogLevel,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)

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

// Callers returns an array of strings containing the function name, source filename and line
// number for the caller of this function and its caller moving up the stack for as many levels as
// are available or the number of levels specified by the levels parameter.
// Set the short parameter to true to only return final element of Function and source file name.
func Callers(skip int, short bool) ([]slog.Source, error) {
	var callers []slog.Source

	// We get the callers as uintptrs.
	fpcs := make([]uintptr, stackDepth)

	// Skip 1 levels to get to the caller of whoever called Callers().
	n := runtime.Callers(skip, fpcs)
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

		caller := slog.Source{Function: funcName, File: sourceFile, Line: lineNumber}
		callers = append(callers, caller)

		if !more {
			break
		}
	}
	// fmt.Printf("callers...\n")
	// for i, c := range callers {
	// 	fmt.Printf("%d: %s(%d) %s\n", i, c.File, c.Line, c.Function)
	// }
	return callers, nil
}

// GetCaller returns the caller of GetCaller 'skip' levels back.
// Set the short parameter to true to only return final element of Function and source file name.
func GetCaller(skip int, short bool) slog.Source {
	callers, err := Callers(skip, short)
	if err != nil {
		return slog.Source{Function: "not available", File: "not available", Line: 0}
	}
	// fmt.Printf("%s(%d) %s\n", callers[0].File, callers[0].Line, callers[0].Function)
	return callers[0]
}

// CallerText generates a string containing caller function, source and line.
func CallerText(skip int) string {
	callerInfo := GetCaller(skip, true)

	return fmt.Sprintf("%s(%d) %s - ", callerInfo.File, callerInfo.Line, callerInfo.Function)
}

// TraceCall traces calls and exit for functions.
func TraceCall() {
	ctx := context.Background()
	TraceLog.Log(ctx, LevelTrace, "Entering function")
}

// TraceExit traces calls and exit for functions.
func TraceExit() {
	ctx := context.Background()
	TraceLog.Log(ctx, LevelTrace, "Exiting function")
}

// TraceCallWithCtx traces calls and exit for functions.
func TraceCallWithCtx(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	TraceLog.Log(ctx, LevelTrace, "Entering function")
}

// TraceExitWithCtx traces calls and exit for functions.
func TraceExitWithCtx(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	TraceLog.Log(ctx, LevelTrace, "Exiting function")
}

// ToJSON is used get data in JSON format.
func ToJSON(log *slog.Logger, data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Warn("failed to convert interface to json", "error", err)

		return err.Error()
	}

	var prettyJSON bytes.Buffer

	err = json.Indent(&prettyJSON, jsonData, "", "  ")
	if err != nil {
		log.Warn("failed to indent json", "error", err)

		return err.Error()
	}

	return prettyJSON.String()
}

// Debug can be used to emit debug output.
func Debug(pattern string, args ...interface{}) {
	if LogLevel <= slog.LevelDebug {
		pattern := CallerText(MyCallersCaller) + pattern
		fmt.Printf(pattern, args...)
	}
}

func ErrorReport(text string, err error) error {
	return fmt.Errorf("%s - %s, %w", CallerText(MyCallersCaller), text, err)
}

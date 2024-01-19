// Package logging contains logging related functions used by multiple packages
package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/go-logr/logr"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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
	four                   = 4
	traceLevelEnvVar       = "TRACE_LEVEL"
)

var TraceLevel = four //nolint:gochecknoglobals //ok

var errNotAvailable = errors.New("caller not availalble")

func init() { //nolint:gochecknoinits //ok
	if tlevel, ok := os.LookupEnv(traceLevelEnvVar); ok {
		var err error
		if TraceLevel, err = strconv.Atoi(tlevel); err != nil {
			fmt.Fprintf(os.Stderr, "invalid 'TRACE_LEVEL' value: %s", tlevel)

			TraceLevel = four
		}
	}
}

// NewLogger returns a logger configured the timestamps format is ISO8601.
func NewLogger(name string, logOpts *zap.Options) logr.Logger {
	encCfg := uzap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zap.Encoder(zapcore.NewJSONEncoder(encCfg))

	return zap.New(zap.UseFlagOptions(logOpts), encoder).WithName(name)
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
func TraceCall(log logr.Logger) {
	callerInfo := GetCaller(MyCaller, true)
	log.V(TraceLevel).Info("Entering function", "function", callerInfo.FunctionName, "source", callerInfo.SourceFile, "line", callerInfo.SourceLine)
}

// TraceExit traces calls and exit for functions.
func TraceExit(log logr.Logger) {
	callerInfo := GetCaller(MyCaller, true)
	log.V(TraceLevel).Info("Exiting function", "function", callerInfo.FunctionName, "source", callerInfo.SourceFile, "line", callerInfo.SourceLine)
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

package miscutils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/nabancard/goutils/pkg/logging"
)

type NewObjParams struct {
	Ctx    context.Context
	Log    *slog.Logger
	LogOut io.Writer
}

type Utils struct {
	MiscUtils
	logger *slog.Logger
}

type MiscUtils interface {
	StringToFile(f *os.File, text string) error
}

func NewMiscUtils(logger *slog.Logger) MiscUtils {
	logging.TraceCall()
	defer logging.TraceExit()

	return &Utils{logger: logger}
}

func (u *Utils) StringToFile(f *os.File, text string) error {
	logging.TraceCall()
	defer logging.TraceExit()

	if _, err := f.WriteString(text); err != nil {
		return logging.ErrorReport(fmt.Sprintf("failed to write to %s file", f.Name()), err)
	}
	return nil
}

// Max returns the maximum of two integers.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// FindInSlice determines if a string is in a slice of strings.
func FindInSlice(value string, slice []string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// TypeExaminer returns a string containing details of an interface type, useful for debugging and understanding data.
func TypeExaminer(t reflect.Type, depth int, data interface{}) string {
	details := fmt.Sprintf("%sType: %-10s kind: %-8s Package: %s\n", strings.Repeat("\t", depth), t.Name(), t.Kind(), t.PkgPath())
	switch t.Kind() { //nolint: exhaustive
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Ptr, reflect.Slice:
		details += fmt.Sprintf("%sContained type...\n", strings.Repeat("\t", depth+1))
		details += TypeExaminer(t.Elem(), depth+1, data)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			details += fmt.Sprintf("%sField: %d: name: %-10s type: %-8s kind: %-12s anon: %-6t package: %s\n",
				strings.Repeat("\t", depth+1), i+1, f.Name, f.Type.Name(), f.Type.Kind().String(), f.Anonymous, f.PkgPath)
			if f.Tag != "" {
				details += fmt.Sprintf("%sTags: %s\n", strings.Repeat("\t", depth+2), f.Tag) //nolint: mnd
			}
			if f.Type.Kind() == reflect.Struct {
				details += TypeExaminer(f.Type, depth+2, data) //nolint: mnd
			}
		}
	}
	return details
}

// IndentJSON is used generate json data indented.
func IndentJSON(data interface{}, offset, indent int) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err.Error()
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonData, strings.Repeat(" ", offset*indent), strings.Repeat(" ", indent))
	if err != nil {
		return err.Error()
	}

	return prettyJSON.String()
}

// CheckError - panics if error.
func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

// PrettyPrint - outputs interface in json.
func PrettyPrint(i interface{}) (string, error) {
	s, err := json.MarshalIndent(i, "", "\t")
	return string(s), err
}

func CopyFile(src, dst string) {
	source, err := os.Open(src)
	CheckError(err)
	defer source.Close()

	destination, err := os.Create(dst)
	CheckError(err)
	defer destination.Close()

	_, err = io.Copy(destination, source)
	CheckError(err)
}

func Exists(name string) (bool, error) {
	_, err := os.Stat(name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func ValidateTimestamp(timeStamp, format string) error {
	_, err := time.Parse(format, timeStamp)
	return err
}

func PromptUsageWarning(o *NewObjParams) {
	fmt.Println("WARNING: this tool will run destructive tasks on the current Cluster.")
	fmt.Println("Please ensure you are authenticated to the correct cluster before proceeding")
	scanner := bufio.NewScanner(os.Stdin)
	color.New(color.Bold, color.FgHiWhite).Printf("Continue with restore: y/N \n")
	scanner.Scan()
	input := scanner.Text()
	yes := "y"
	if input != yes {
		LogErrorFatal(o, "exiting script")
		os.Exit(1)
	}
}

func LogWarning(o *NewObjParams, text string) {
	warnOutput := color.New(color.FgYellow).SprintFunc()
	o.Log.Warn(warnOutput(text))
}

func LogInfo(o *NewObjParams, text string) {
	infoOutput := color.New(color.FgGreen).SprintFunc()
	o.Log.Info(infoOutput(text))
}

func LogInfoBlue(o *NewObjParams, text string) {
	infoOutput := color.New(color.Bold, color.FgBlue).SprintFunc()
	o.Log.Info(infoOutput(text))
}

func LogError(o *NewObjParams, text string) {
	errorOutput := color.New(color.FgRed).SprintFunc()
	o.Log.Error(errorOutput(text))
}

func LogErrorFatal(o *NewObjParams, text string) {
	errorOutput := color.New(color.FgRed).SprintFunc()
	o.Log.Log(o.Ctx, logging.LevelFatal, errorOutput(text))
}

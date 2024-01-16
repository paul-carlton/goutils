package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/kylelemons/godebug/diff"

	"github.com/paul-carlton/goutils/pkg/logging"
)

// Test utilities

// CopyObjectStatus copys an ObjectStatus object leaving Fields element empty.
func CopyObjectStatus(objStatus *ObjectStatus) *ObjectStatus {
	return &ObjectStatus{
		Object:     objStatus.Object,
		GetField:   objStatus.GetField,
		SetField:   objStatus.SetField,
		CallMethod: objStatus.CallMethod,
		Fields:     make(Fields, len(objStatus.Fields)),
	}
}

// SetObjStatusFields sets the values in the FieldInfo struct, used to update a template ObjectStatus with values for a given test.
func SetObjStatusFields(t *testing.T, objStatus *ObjectStatus, fieldValues map[string]interface{}, retain bool) *ObjectStatus {
	newObj := CopyObjectStatus(objStatus)

	if retain {
		for fieldName, fieldInfo := range objStatus.Fields {
			newObj.Fields[fieldName] = FieldInfo{
				Comparer:     fieldInfo.Comparer,
				Reporter:     fieldInfo.Reporter,
				GetterMethod: fieldInfo.GetterMethod,
				SetterMethod: fieldInfo.SetterMethod,
				FieldValue:   fieldInfo.FieldValue,
			}
		}
	}

	for fieldName, fieldValue := range fieldValues {
		info, ok := objStatus.Fields[fieldName]

		if !ok {
			t.Logf("field: %s not found in object status fields", fieldName)

			continue
		}

		newObj.Fields[fieldName] = FieldInfo{
			Comparer:     info.Comparer,
			Reporter:     info.Reporter,
			GetterMethod: info.GetterMethod,
			SetterMethod: info.SetterMethod,
			FieldValue:   fieldValue,
		}
	}

	return newObj
}

// IsPointer returns true if a variable is a pointer.
func IsPointer(i interface{}) bool {
	return reflect.ValueOf(i).Type().Kind() == reflect.Ptr
}

// GetPointerValue returns what a pointer points to.
func GetPointerValue(i interface{}) interface{} {
	if !IsPointer(i) {
		return i
	}

	return reflect.Indirect(reflect.ValueOf(i)).Interface()
}

// CheckFieldValue will check if a field value is equal to the expected value and set the test to failed if not.
func CheckFieldValue(u TestUtil, fieldName string, fieldInfo FieldInfo) bool {
	test := u.TestData()
	t := u.Testing()

	if test.ObjStatus == nil || test.ObjStatus.GetField == nil {
		if u.Verbose() {
			t.Logf("object Get Field function not set, skipping check of fields values")
		}

		return true
	}

	actual := test.ObjStatus.GetField(t, test.ObjStatus.Object, fieldName)
	expected := fieldInfo.FieldValue
	passed := false

	if fieldInfo.Comparer == nil {
		passed = u.FieldComparer(fieldName, actual, expected)
	} else {
		passed = fieldInfo.Comparer(u, fieldName, actual, expected)
	}

	if !passed || u.FailTests() {
		u.FieldReporter(fieldName, actual, expected)
	}

	return passed
}

// CheckFieldGetter will the getter function of a field and check the value is equal to the expected value and set the test to failed if not.
func CheckFieldGetter(u TestUtil, fieldName string, fieldInfo FieldInfo) bool {
	test := u.TestData()
	t := u.Testing()

	if test.ObjStatus == nil || test.ObjStatus.CallMethod == nil {
		if u.Verbose() {
			t.Logf("object CallMethod function not set, skipping check of fields getters")
		}

		return true
	}

	if len(fieldInfo.GetterMethod) > 0 {
		results := test.ObjStatus.CallMethod(t, test.ObjStatus.Object, fieldInfo.GetterMethod, []interface{}{})
		actual := results[0]
		expected := fieldInfo.FieldValue
		passed := false

		if fieldInfo.Comparer == nil {
			passed = u.FieldComparer(fieldName, actual, expected)
		} else {
			passed = fieldInfo.Comparer(u, fieldName, actual, expected)
		}

		if !passed || u.FailTests() {
			u.FieldReporter(fieldName, actual, expected)
		}

		return passed
	}

	if u.Verbose() {
		t.Logf("field getter method not set, skipping check of field: %s getter", fieldName)
	}

	return true
}

// CheckFieldsValue will get the value of object fields and check the expected values are equal to the actual value and set the test to failed if not.
func CheckFieldsValue(u TestUtil) bool {
	test := u.TestData()
	t := u.Testing()

	if test.ObjStatus == nil || test.ObjStatus.GetField == nil {
		if u.Verbose() {
			t.Logf("object Get Field function not set, skipping check of fields values")
		}

		return true
	}

	for fieldName, fieldInfo := range test.ObjStatus.Fields {
		if !CheckFieldValue(u, fieldName, fieldInfo) {
			return false
		}
	}

	return true
}

// CheckFieldsGetter will call the getter functions on an object to check the expected values are equal to the actual value and set the test to failed if not.
func CheckFieldsGetter(u TestUtil) bool {
	test := u.TestData()
	t := u.Testing()

	if test.ObjStatus == nil || test.ObjStatus.CallMethod == nil {
		if u.Verbose() {
			t.Logf("object call method function not set, skipping check of fields getter functions")
		}

		return true
	}

	for fieldName, fieldInfo := range test.ObjStatus.Fields {
		if !CheckFieldGetter(u, fieldName, fieldInfo) {
			return false
		}
	}

	return true
}

// CastToStringList casts an interface to a []string.
func CastToStringList(t *testing.T, data interface{}) []string {
	strList, ok := data.([]string)
	if !ok {
		t.Fatalf("failed to cast interface to []string")

		return nil
	}

	return strList
}

// CastToMapStringString casts an interface to a map[string]string.
func CastToMapStringString(t *testing.T, data interface{}) map[string]string {
	strMap, ok := data.(map[string]string)
	if !ok {
		t.Fatalf("failed to cast interface to map[string]string")

		return nil
	}

	return strMap
}

// UnsetEnvs unsets a list of environmental variables.
func UnsetEnvs(t *testing.T, names []string) {
	for _, env := range names {
		UnsetEnv(t, env)

		if t.Failed() {
			return
		}
	}
}

// MapKeysToList returns the keys of a map[string]string as a list.
func MapKeysToList(envsMap map[string]string) []string {
	keys := make([]string, 0, len(envsMap))

	for k := range envsMap {
		keys = append(keys, k)
	}

	return keys
}

// SetEnv sets an environmental variable.
func SetEnv(t *testing.T, envName, envValue string) {
	if err := os.Setenv(envName, envValue); err != nil {
		t.Fatalf("failed to set environmental variable: %s to %s, %s", envName, envValue, err)

		return
	}

	t.Logf("environmental variable: %s, set to %s", envName, envValue)
}

// UnsetEnv unsets an environmental variable.
func UnsetEnv(t *testing.T, envName string) {
	if err := os.Unsetenv(envName); err != nil {
		t.Fatalf("failed to unset environmental variable: %s , %s", envName, err)

		return
	}
}

// SetEnvs sets a number of environmental variables.
func SetEnvs(t *testing.T, envSettings map[string]string) {
	for name, val := range envSettings {
		SetEnv(t, name, val)

		if t.Failed() {
			return
		}
	}
}

// ContainsStringArray checks items in two string arrays verifying that the second string array is
// contained in the first.
func ContainsStringArray(one, two []string, first bool) bool {
	if len(one) == 0 && len(two) == 0 { // Both empty return true
		return true
	}

	if len(two) == 0 {
		return false
	}

	offset := 0

	for index, field := range one {
		if field == two[offset] {
			offset++
			index++

			if index < len(one) && offset < len(two) {
				return ContainsStringArray(one[index:], two[offset:], true)
			}

			if offset == len(two) {
				return true
			}
		}

		if first {
			return false
		}
	}

	return false
}

// ReadBuf reads lines from a buffer into a string array.
func ReadBuf(out io.Reader) *[]string {
	output := []string{}

	for {
		p := make([]byte, 1024000)
		n, err := out.Read(p)

		if n > 0 {
			lines := strings.Split(string(p[:n]), "\n")
			output = append(output, lines...)
		}

		if err != nil {
			break
		}
	}

	// If last line is empty then remove it
	if len(output) > 0 && len(output[len(output)-1]) == 0 {
		output = output[:len(output)-1]
	}

	return &output
}

// DisplayStrings returns a []string as a single string, one element per line.
func DisplayStrings(strs []string) string {
	output := ""
	newline := ""

	for index, str := range strs {
		output = fmt.Sprintf("%s%s%d - %s", output, newline, index, str)
		newline = "\n"
	}

	return output
}

// HandlePanic handles a panic in test code calling testing.T.Fatal() with interface returned by recover().
func HandlePanic(t *testing.T) {
	if a := recover(); a != nil {
		t.Fatalf("%s%s\nstack trace... \n%s", logging.CallerText(logging.MyCallersCallersCaller), a, string(debug.Stack()))
	}
}

// CallMethod calls a method on an object and returns the results.
func CallMethod(t *testing.T, obj interface{}, methodName string, params []interface{}) []interface{} {
	defer HandlePanic(t)

	ro := reflect.ValueOf(obj).MethodByName(methodName)
	p := []reflect.Value{}

	for _, param := range params {
		p = append(p, reflect.ValueOf(param))
	}

	values := ro.Call(p)
	results := make([]interface{}, len(values))

	for index, value := range values {
		results[index] = value.Interface()
	}

	return results
}

// ContainsStrings checks that all elements in a list of strings are contained in another string.
func ContainsStrings(expected []string, result string) bool {
	for _, text := range expected {
		if !strings.Contains(result, text) {
			return false
		}
	}

	return true
}

// CheckError implements the CheckTestI interface, verifying that a error contains expected text strings.
// It then calls the default check function to verify other results, if any.
func CheckError(u TestUtil) bool {
	defer HandlePanic(u.Testing())
	t := u.Testing()
	test := u.TestData()

	expected, ok := test.Expected[len(test.Expected)-1].([]string)
	if !ok {
		panic("failed to cast expected to []string")
	}

	result := test.Results[len(test.Results)-1].(error).Error()

	if !ContainsStrings(expected, result) {
		t.Fatalf("\nTest: %d, %s, error does not contain expected text\nGot.....: %s\nExpected: %s",
			test.Number, test.Description, result, expected)

		return false
	}

	if len(test.Results) > 1 {
		test.Results = test.Results[0 : len(test.Results)-1]
		test.Expected = test.Expected[0 : len(test.Expected)-1]

		return DefaultCheckFunc(u)
	}

	return true
}

// CheckNotNil is passed two interfaces and checks if both or neither are nil.
// If both are nil it returns true, if neither are nil it returns false.
// If one is nil and the other is not it reports the difference and returns nil.
func CheckNotNil(u TestUtil, name string, a, e interface{}) bool {
	t := u.Testing()
	test := u.TestData()

	if a == nil || e == nil {
		if a != nil {
			t.Fatalf("\nTest: %d, %s\nField....: %s\nActual value is nil and expected is not", test.Number, test.Description, name)
		}

		if e != nil {
			t.Fatalf("\nTest: %d, %s\nField....: %s\nExpected value is nil and actual is not", test.Number, test.Description, name)
		}

		return false
	}

	return true
}

// CompareJSON compares values by converting to json.
func CompareJSON(u TestUtil, name string, actual, expected interface{}) bool {
	return CompareAsJSON(u.Testing(), actual, expected)
}

// CompareReflectDeepEqual compares values using reflect.DeepEqual().
func CompareReflectDeepEqual(u TestUtil, name string, actual, expected interface{}) bool {
	return reflect.DeepEqual(actual, expected)
}

// ReportSpew uses spew to report the differences between two values.
// It displays the actual and expected values as well as the differences between them.
func ReportSpew(u TestUtil, name string, actual, expected interface{}) {
	t := u.Testing()
	test := u.TestData()

	spewActual := spew.Sdump(actual)
	spewExpected := spew.Sdump(expected)

	if len(name) == 0 {
		ReportCallSpew(u)

		return
	}

	t.Errorf("\nTest: %d, %s\nField....: %s\nGot.....: %s\nExpected: %s\nDiff....: %s",
		test.Number, test.Description, name, spewActual, spewExpected, diff.Diff(spewActual, spewExpected))
}

// ReportCallSpew uses spew to report on result of a function or method call.
// It displays the input, result and expected result as well as the differences between the actual and expected results.
func ReportCallSpew(u TestUtil) {
	t := u.Testing()
	test := u.TestData()
	spewActual := spew.Sdump(test.Results)
	spewExpected := spew.Sdump(test.Expected)

	t.Errorf("\nTest: %d, %s\nInput...: %s\nGot.....: %s\nExpected: %s\nDiff....: %s",
		test.Number, test.Description, spew.Sdump(test.Inputs), spewActual, spewExpected, diff.Diff(spewActual, spewExpected))
}

// ReportJSON uses json to report the differences between two values.
// It displays the actual and expected values as well as the differences between them.
func ReportJSON(u TestUtil, name string, actual, expected interface{}) {
	t := u.Testing()
	test := u.TestData()
	jsonActual := ToJSON(t, actual)
	jsonExpected := ToJSON(t, expected)

	if len(name) == 0 {
		ReportCallJSON(u)

		return
	}

	t.Errorf("\nTest: %d, %s\nField....: %s\nGot.....: %s\nExpected: %s\nDiff....: %s",
		test.Number, test.Description, name, jsonActual, jsonExpected, diff.Diff(jsonActual, jsonExpected))
}

// ReportCallJSON uses json to report on result of a function or method call.
// It displays the input, result and expected result as well as the differences between the actual and expected results.
func ReportCallJSON(u TestUtil) {
	t := u.Testing()
	test := u.TestData()
	jsonActual := ToJSON(t, test.Results)
	jsonExpected := ToJSON(t, test.Expected)

	t.Errorf("\nTest: %d, %s\nInput...: %s\nGot.....: %s\nExpected: %s\nDiff....: %s",
		test.Number, test.Description, ToJSON(t, test.Inputs), jsonActual, jsonExpected, diff.Diff(jsonActual, jsonExpected))
}

// ToJSON is used get an interface in JSON format.
func ToJSON(t *testing.T, data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to convert interface to json, %s", err)

		return err.Error()
	}

	var prettyJSON bytes.Buffer

	err = json.Indent(&prettyJSON, jsonData, "", "  ")
	if err != nil {
		t.Fatalf("failed to indent json, %s", err)

		return err.Error()
	}

	return prettyJSON.String()
}

// CompareAsJSON compares two interfaces as JSON.
func CompareAsJSON(t *testing.T, one, two interface{}) bool {
	if one == nil && two == nil {
		return true
	}

	return ToJSON(t, one) == ToJSON(t, two)
}

// BoolPtr returns a pointer to a bool.
func BoolPtr(b bool) *bool {
	return &b
}

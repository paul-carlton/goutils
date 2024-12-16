// Package testutils provides a framework and helper functions for use during unit testing.
package testutils

import (
	"fmt"
	"os"
	"testing"

	"github.com/nabancard/goutils/pkg/logging"
)

type (
	// PrepTestI defines function to be called before running a test.
	PrepTestI func(u TestUtil)
	// CheckTestI defines function to be called after test to check result.
	CheckTestI func(u TestUtil) bool
	// ReportDiffI defines the report difference function interface.
	ReportDiffI func(u TestUtil, name string, actual, expected interface{})
	// ComparerI defines the comparer function interface.
	ComparerI func(u TestUtil, name string, actual, expected interface{}) bool
	// PostTestI defines function to be called after running a test.
	PostTestI func(u TestUtil)

	// GetFieldFunc is the function to call to get the value of a field of an object.
	GetFieldFunc func(t *testing.T, obj interface{}, fieldName string) interface{}
	// SetFieldFunc is the function to call to set the value of a field of an object.
	SetFieldFunc func(t *testing.T, obj interface{}, fieldName string, value interface{})
	// CallMethodFunc is the function to call a method on an object.
	CallMethodFunc func(t *testing.T, obj interface{}, methodName string, values []interface{}) []interface{}

	// FieldInfo holds information about a field of a struct.
	FieldInfo struct {
		Reporter     ReportDiffI `json:"reporter,omitempty"` // Function to do field specific reporting of differences, nil if not set.
		Comparer     ComparerI   `json:"comparer,omitempty"` // Function to do field specific compare, nil if not set.
		GetterMethod string      `json:"getter,omitempty"`   // The method to get the value, nil if no getter method.
		SetterMethod string      `json:"setter,omitempty"`   // The method to get the value, nil if no setter method.
		FieldValue   interface{} `json:"value"`              // The value to set or expected value to verify.
	}

	// Fields is a map of field names to information about the field.
	Fields map[string]FieldInfo

	// ObjectStatus hold details of the object under test.
	// This can be used to verify the internal state of an object after a test.
	ObjectStatus struct {
		Object     interface{}    // The object or interface under test, this needs to be set during test before calling post test actions.
		GetField   GetFieldFunc   // The function to call to get a field value.
		SetField   SetFieldFunc   // The function to call to set a field value.
		CallMethod CallMethodFunc // The function to call a method on an object.
		Fields     Fields         // The fields of an object.
	}

	// DefTest generic test data structure.
	DefTest struct {
		Number      int           // Test number.
		Description string        // Test description.
		Result      bool          // The result of the test.
		EnvVars     []string      // List of environmental variable to be reset at the start of each test
		Config      interface{}   // Test configuration information to be used by test function or custom pre/post test functions.
		Inputs      []interface{} // Test inputs.
		Expected    []interface{} // Test expected results.
		Results     []interface{} // Test results.
		ObjStatus   *ObjectStatus // Details of object under test including field names and expected values, used by CheckFunc to verify values.
		// PrepFunc is function to be called before a test, leave unset to call default.
		PrepFunc PrepTestI
		// PostFunc is function to be called after a test, leave unset to call default.
		PostFunc PostTestI
		// CheckFunc is function to be called to check a test results, leave unset to call default.
		// Default compares actual results with expected results and verifies object status.
		CheckFunc CheckTestI
		// ResultsCompareFunc is function to be called to compare a test results, leave unset to call default.
		// Default compares actual results with expected results using reflect.DeepEqual().
		ResultsCompareFunc ComparerI
		// ResultsReportFunc is function to be called to report difference in test results, leave unset to call default - which uses spew.Sdump().
		ResultsReportFunc ReportDiffI
		// FieldCompareFunc is function to be called to compare a field values, leave unset to call default.
		// Default compares actual results with expected results using reflect.DeepEqual().
		FieldCompareFunc ComparerI
		// FieldCompareFunc is function to be called to report difference in field values, leave unset to call default - which uses spew.Sdump().
		FieldReportFunc ReportDiffI
	}

	// TestUtil the interface used to provide testing utilities.
	TestUtil interface {
		CallPrepFunc()                 // Call the custom or default test preparation function.
		CallPostFunc()                 // Call the custom or default test tidy up function.
		CallCheckFunc() bool           // Call the custom or default test checking function.
		Testing() *testing.T           // testing object.
		SetFailTests(value bool)       // Set the fail test setting to verify test check reporting.
		FailTests() bool               // Get the fail test setting.
		SetVerbose(value bool)         // Set the verbose setting.
		Verbose() bool                 // Get the verbose setting.
		Result() bool                  // Return the result of the test.
		SetResult(value bool)          // Set the result of the test.
		SetTestData(testData *DefTest) // Set the test data.
		TestData() *DefTest            // Get the test data.
		// ResultsComparer calls the specified comparer, default checking function calls this to call test data's CompareFunc or CompareReflectDeepEqual if not set.
		ResultsComparer() bool
		// FieldComparer calls the field comparer, default checking function calls this to call test data's CompareFunc or CompareReflectDeepEqual if not set.
		FieldComparer(name string, actual, expected interface{}) bool
		// ResultsReporter calls the specified reporter, default checking function calls this to call test data's ResultsReportFunc or ReportSpew if not set.
		ResultsReporter()
		// FieldReporter calls the specified reporter, default checking function calls this to call test data's ReportFieldsFunc or ReportSpew if not set.
		FieldReporter(name string, actual, expected interface{})
	}

	// testUtil is used to hold configuration information for testing.
	testUtil struct {
		TestUtil             // TestUtil interface that operates on this object.
		t         *testing.T // Testing object.
		testData  *DefTest   // The definition of this test.
		failTests bool       // Set to make default test check function reported retrun false to test report function.
		verbose   bool       // Set to make testutils more verbose
	}
)

// NewTestUtil retruns a new TestUtil interface.
func NewTestUtil(t *testing.T, testData *DefTest) TestUtil {
	u := &testUtil{failTests: false}
	u.t = t
	u.testData = testData

	_, present := os.LookupEnv("TESTUTILS_FAIL")
	if present {
		u.failTests = true
	}

	_, present = os.LookupEnv("TESTUTILS_VERBOSE")
	if present {
		u.verbose = true
	}

	return u
}

// CallPrepFunc calls the pre test setup function.
func (u *testUtil) CallPrepFunc() {
	DefaultPrepFunc(u)

	if u.testData.PrepFunc != nil {
		u.testData.PrepFunc(u)
	}
}

// CallPostFunc calls the post test setup function.
func (u *testUtil) CallPostFunc() {
	DefaultPostFunc(u)

	if u.testData.PostFunc != nil {
		u.testData.PostFunc(u)
	}
}

// CallCheckTestsFunc calls the check test result function.
func (u *testUtil) CallCheckFunc() bool {
	if u.testData.CheckFunc == nil {
		return DefaultCheckFunc(u)
	}

	return u.testData.CheckFunc(u)
}

// Testing returns the testing object.
func (u *testUtil) Testing() *testing.T {
	return u.t
}

// SetVerbose sets the verbose flag.
func (u *testUtil) SetVerbose(value bool) {
	u.verbose = value
}

// Verbose gets the verbose flag.
func (u *testUtil) Verbose() bool {
	return u.verbose
}

// Result returns the result of the test.
func (u *testUtil) Result() bool {
	return u.TestData().Result
}

// SetResult sets the tests result.
func (u *testUtil) SetResult(value bool) {
	u.TestData().Result = value
}

// SetFailTests sets the fail tests flag.
func (u *testUtil) SetFailTests(value bool) {
	u.failTests = value
}

// FailTests returns the fail test setting.
func (u *testUtil) FailTests() bool {
	return u.failTests
}

// SetTestData sets the test data.
func (u *testUtil) SetTestData(testData *DefTest) {
	u.testData = testData
}

// TestData returns the test data.
func (u *testUtil) TestData() *DefTest {
	return u.testData
}

// DefaultPrepFunc is the default pre test function.
func DefaultPrepFunc(u TestUtil) {
	logging.TraceCall()
	defer logging.TraceExit()

	t := u.Testing()
	test := u.TestData()
	UnsetEnvs(t, test.EnvVars)
}

// DefaultPostFunc is the default post test function.
func DefaultPostFunc(_ TestUtil) {
}

func (u *testUtil) ResultsReporter() {
	logging.TraceCall()
	defer logging.TraceExit()

	test := u.TestData()
	if test.ResultsReportFunc == nil {
		if test.FieldReportFunc == nil {
			ReportCallSpew(u)
			return
		}
		for index := range len(test.Results) {
			test.FieldReportFunc(u, fmt.Sprintf("%d", index), test.Results[index], test.Expected[index])
		}
		return
	}

	test.ResultsReportFunc(u, "", test.Results, test.Expected)
}

func (u *testUtil) FieldReporter(name string, actual, expected interface{}) {
	logging.TraceCall()
	defer logging.TraceExit()

	test := u.TestData()
	if test.FieldReportFunc == nil {
		ReportSpew(u, name, actual, expected)

		return
	}

	test.FieldReportFunc(u, name, actual, expected)
}

func (u *testUtil) ResultsComparer() bool {
	logging.TraceCall()
	defer logging.TraceExit()

	test := u.TestData()
	t := u.Testing()
	passed := false

	if test.ResultsCompareFunc == nil { //nolint: nestif
		if test.FieldCompareFunc == nil {
			passed = CompareReflectDeepEqual(test.Results, test.Expected)
		} else {
			for index := range len(test.Results) {
				passed = test.FieldCompareFunc(u, fmt.Sprintf("%d", index), test.Results[index], test.Expected[index])
				if !passed {
					if u.Verbose() {
						t.Logf("result field; %d, failed", index)
					}
					break
				}
			}
		}
	} else {
		passed = test.ResultsCompareFunc(u, "", test.Results, test.Expected)
	}

	if !passed || u.FailTests() {
		u.ResultsReporter()
	}

	return passed
}

func (u *testUtil) FieldComparer(name string, actual, expected interface{}) bool {
	logging.TraceCall()
	defer logging.TraceExit()

	test := u.TestData()
	if test.FieldCompareFunc == nil {
		return CompareReflectDeepEqual(actual, expected)
	}

	u.SetResult(test.FieldCompareFunc(u, name, actual, expected))
	if u.Verbose() {
		t := u.Testing()
		t.Logf("Field comparer returned: %t", u.Result())
	}
	return u.Result()
}

// DefaultCheckFunc is the default check test function that compares actual and expected.
func DefaultCheckFunc(u TestUtil) bool {
	logging.TraceCall()
	defer logging.TraceExit()

	result := u.ResultsComparer() && CheckObjStatusFunc(u)
	u.SetResult(result)
	if u.Verbose() {
		t := u.Testing()
		t.Logf("Test result: %t", u.Result())
	}
	return result
}

// CheckObjStatusFunc checks object fields values against expected and report if different.
func CheckObjStatusFunc(u TestUtil) bool {
	logging.TraceCall()
	defer logging.TraceExit()

	return CheckFieldsValue(u) && CheckFieldsGetter(u)
}

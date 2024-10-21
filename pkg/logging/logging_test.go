package logging_test

import (
	"log/slog"
	"testing"

	"github.com/paul-carlton/goutils/pkg/logging"
	"github.com/paul-carlton/goutils/pkg/testutils"
)

func TestGetObjLabel(t *testing.T) {
	tests := []*testutils.DefTest{
		{
			Number:      1,
			Description: "",
			Inputs: []interface{}{
				slog.Attr{
					Key: slog.SourceKey,
					Value: slog.AnyValue(&slog.Source{
						Function: "a.func",
						File:     "/a/b/c/x.go",
						Line:     123,
					}),
				},
			},
			Expected: []interface{}{
				slog.Attr{
					Key: slog.SourceKey,
					Value: slog.AnyValue(&slog.Source{
						Function: "a.func",
						File:     "x.go",
						Line:     123,
					}),
				},
			},
		},
	}

	testFunc := func(t *testing.T, testData *testutils.DefTest) bool {
		u := testutils.NewTestUtil(t, testData)

		u.CallPrepFunc()

		result := logging.SetSourceName(testData.Inputs[0].(slog.Attr))
		testData.Results = []interface{}{result}

		return u.CallCheckFunc()
	}

	for _, test := range tests {
		if !testFunc(t, test) {
			return
		}
	}
}

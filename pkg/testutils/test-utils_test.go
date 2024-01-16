package testutils_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/paul-carlton/goutils/pkg/testutils"
)

func TestContainsStringArray(t *testing.T) {
	tests := []struct {
		testNum  int
		one      []string
		two      []string
		expected bool
	}{
		{1, []string{"a", "b", "c", "d"}, []string{"b", "c", "d"}, true},
		{2, []string{"a", "b", "c", "d"}, []string{"a", "b", "c", "d"}, true},
		{3, []string{"a", "b", "c", "d"}, []string{"b", "d"}, false},
		{4, []string{"a", "b", "c", "d"}, []string{"a", "b"}, true},
		{5, []string{"a"}, []string{}, false},
		{6, []string{}, []string{}, true},
		{7, []string{"a"}, []string{"a", "b"}, false},
	}

	for _, test := range tests {
		result := testutils.ContainsStringArray(test.one, test.two, false)
		if result != test.expected {
			t.Errorf("\nTest: %d\narray one:\n%+v\narray two:\n%+v\nExpected: %t\nGot.....: %t",
				test.testNum, test.one, test.two, test.expected, result)
		}
	}
}

func TestReadBuf(t *testing.T) {
	tests := []struct {
		expected *[]string
	}{{&[]string{"b", "c", "d"}}}

	for _, test := range tests {
		buffer := &bytes.Buffer{}

		for _, input := range *test.expected {
			buffer.WriteString(fmt.Sprintf("%s\n", input))
		}

		result := testutils.ReadBuf(buffer)
		if !testutils.ContainsStringArray(*result, *test.expected, true) {
			t.Errorf("\nExpected:\n%+v\nGot.....:\n%+v", test.expected, result)
		}
	}
}

func TestDisplayStrings(t *testing.T) {
	tests := []struct {
		one      []string
		expected string
	}{
		{[]string{"a", "b", "c"}, "0 - a\n1 - b\n2 - c"},
	}

	for _, test := range tests {
		result := testutils.DisplayStrings(test.one)
		if result != test.expected {
			t.Errorf("\ninput:\n%+v\nExpected:\n%s\nGot.....:\n%s",
				test.one, test.expected, result)
		}
	}
}

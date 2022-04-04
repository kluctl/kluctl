package versions

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParse(t *testing.T) {

	type testCase struct {
		s string
		expectedFilter LatestVersionFilter
		expectedErr error
	}

	testCases := []testCase{
		{s: "regex('a*')", expectedFilter: NewRegexVersionFilterMust("a*")},
		{s: "regex(\"a*\")", expectedFilter: NewRegexVersionFilterMust("a*")},
		{s: "regex(a*\")", expectedErr: fmt.Errorf("unexpected token 42, expected ')' or ','")},
		{s: "semver()", expectedFilter: NewLooseSemVerVersionFilter(false)},
		{s: "semver(allow_no_nums)", expectedErr: fmt.Errorf("invalid value for arg allow_no_nums, must be a bool")},
		{s: "semver(allow_no_nums=false)", expectedFilter: NewLooseSemVerVersionFilter(false)},
		{s: "semver(allow_no_nums=true)", expectedFilter: NewLooseSemVerVersionFilter(true)},
		{s: "prefix('')", expectedFilter: NewPrefixVersionFilterMust("", NewLooseSemVerVersionFilter(false))},
		{s: "prefix('a')", expectedFilter: NewPrefixVersionFilterMust("a", NewLooseSemVerVersionFilter(false))},
		{s: "prefix('a', 'b')", expectedErr: fmt.Errorf("unexpected argument type for , expected -2, got -6")},
		{s: "prefix('a', regex('a*'))", expectedFilter: NewPrefixVersionFilterMust("a", NewRegexVersionFilterMust("a*"))},
		{s: "prefix('a', suffi=regex('a*'))", expectedErr: fmt.Errorf("unkown arg suffi")},
		{s: "prefix('a', suffix=regex('a*'))", expectedFilter: NewPrefixVersionFilterMust("a", NewRegexVersionFilterMust("a*"))},
		{s: "number()", expectedFilter: NewNumberVersionFilter()},
		{s: "number(a=1)", expectedErr: fmt.Errorf("unexpected token -2, expected (")},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.s, func(t *testing.T) {
			f, err := ParseLatestVersion(tc.s)
			assert.Equal(t, tc.expectedFilter, f)
			if tc.expectedErr != nil {
				assert.Error(t, err, tc.expectedErr)
			} else if err != nil {
				assert.Fail(t, "unexpected error", err)
			}
		})
	}
}

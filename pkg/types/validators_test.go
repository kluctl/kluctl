package types

import (
	"fmt"
	gittypes "github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func TestValidateGitProjectSubDir(t *testing.T) {

	var subDirItems = []struct {
		have string
		want bool
	}{
		{"subDir", true},
		{"subDir/../subDir", true},
		{"subDir?", false},            // wrong characters
		{"subDir*/Another", false},    // wrong characters
		{"subDir/../sub?D|ir", false}, // wrong characters
	}

	validate := validator.New()
	validate.RegisterStructValidation(ValidateGitProject, GitProject{})

	for _, item := range subDirItems {
		u := gittypes.ParseGitUrlMust("http://example.com/test")
		gp := GitProject{Url: *u, SubDir: item.have}
		err := validate.Struct(gp)
		if item.want {
			assert.Nil(t, err)
		} else {
			assert.Error(t, err)
		}
	}
}

func TestValidateVarsSource(t *testing.T) {
	validate := validator.New()
	validate.RegisterStructValidation(ValidateVarsSource, VarsSource{})

	type testCase struct {
		vs VarsSource
		e  string
	}

	tests := []testCase{
		{vs: VarsSource{Values: uo.New()}},                             // no error
		{vs: VarsSource{Values: uo.New(), Sensitive: utils.Ptr(true)}}, // no error
		{vs: VarsSource{}, e: "unknown vars source type"},
		{vs: VarsSource{Values: uo.New(), File: utils.Ptr("test")}, e: "more then one vars source type"},
		{vs: VarsSource{Values: uo.New(), File: utils.Ptr("test"), SystemEnvVars: uo.New()}, e: "more then one vars source type"},
	}

	for i, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			err := validate.Struct(&tc.vs)
			if tc.e == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.e)
			}
		})
	}
}

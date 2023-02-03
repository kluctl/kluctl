package types

import (
	"testing"

	validator "github.com/go-playground/validator/v10"
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
		gp := GitProject{SubDir: item.have}
		err := validate.Struct(gp)
		if item.want {
			assert.Nil(t, err)
		} else {
			assert.Error(t, err)
		}
	}
}

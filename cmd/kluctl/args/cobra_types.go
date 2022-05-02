package args

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
)

type existingPathType string

func (s *existingPathType) Set(val string) error {
	if val != "-" {
		val = utils.ExpandPath(val)
	}
	if !utils.Exists(val) {
		return fmt.Errorf("%s does not exist", val)
	}
	*s = existingPathType(val)
	return nil
}
func (s *existingPathType) Type() string {
	return "existingpath"
}

func (s *existingPathType) String() string { return string(*s) }

type existingFileType string

func (s *existingFileType) Set(val string) error {
	if val != "-" {
		val = utils.ExpandPath(val)
	}
	if !utils.Exists(val) {
		return fmt.Errorf("%s does not exist", val)
	}
	if utils.IsDirectory(val) {
		return fmt.Errorf("%s exists but is a directory", val)
	}
	*s = existingFileType(val)
	return nil
}
func (s *existingFileType) Type() string {
	return "existingfile"
}

func (s *existingFileType) String() string { return string(*s) }

type existingDirType string

func (s *existingDirType) Set(val string) error {
	if val != "-" {
		val = utils.ExpandPath(val)
	}
	if !utils.Exists(val) {
		return fmt.Errorf("%s does not exist", val)
	}
	if !utils.IsDirectory(val) {
		return fmt.Errorf("%s exists but is not a directory", val)
	}
	*s = existingDirType(val)
	return nil
}
func (s *existingDirType) Type() string {
	return "existingdir"
}

func (s *existingDirType) String() string { return string(*s) }

type pathType string

func (s *pathType) Set(val string) error {
	if val != "-" {
		val = utils.ExpandPath(val)
	}
	if !utils.Exists(val) {
		return fmt.Errorf("%s does not exist", val)
	}
	if !utils.IsDirectory(val) {
		return fmt.Errorf("%s exists but is not a directory", val)
	}
	*s = pathType(val)
	return nil
}
func (s *pathType) Type() string {
	return "path"
}

func (s *pathType) String() string { return string(*s) }

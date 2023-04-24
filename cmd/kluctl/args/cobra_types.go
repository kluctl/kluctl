package args

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
)

type ExistingPathType string

func (s *ExistingPathType) Set(val string) error {
	if val != "-" {
		val = utils.ExpandPath(val)
	}
	if !utils.Exists(val) {
		return fmt.Errorf("%s does not exist", val)
	}
	*s = ExistingPathType(val)
	return nil
}
func (s *ExistingPathType) Type() string {
	return "existingpath"
}

func (s *ExistingPathType) String() string { return string(*s) }

type ExistingFileType string

func (s *ExistingFileType) Set(val string) error {
	if val != "-" {
		val = utils.ExpandPath(val)
	}
	if !utils.Exists(val) {
		return fmt.Errorf("%s does not exist", val)
	}
	if utils.IsDirectory(val) {
		return fmt.Errorf("%s exists but is a directory", val)
	}
	*s = ExistingFileType(val)
	return nil
}
func (s *ExistingFileType) Type() string {
	return "existingfile"
}

func (s *ExistingFileType) String() string { return string(*s) }

type ExistingDirType string

func (s *ExistingDirType) Set(val string) error {
	if val != "-" {
		val = utils.ExpandPath(val)
	}
	if !utils.Exists(val) {
		return fmt.Errorf("%s does not exist", val)
	}
	if !utils.IsDirectory(val) {
		return fmt.Errorf("%s exists but is not a directory", val)
	}
	*s = ExistingDirType(val)
	return nil
}
func (s *ExistingDirType) Type() string {
	return "existingdir"
}

func (s *ExistingDirType) String() string { return string(*s) }

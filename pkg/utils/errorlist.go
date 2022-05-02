package utils

type errorList struct {
	errors []error
}

func NewErrorListOrNil(errors []error) error {
	if len(errors) == 0 {
		return nil
	}
	return &errorList{errors: errors}
}

func (el *errorList) Error() string {
	s := ""
	for _, err := range el.errors {
		if len(s) != 0 {
			s += "\n"
		}
		s += err.Error()
	}
	return s
}

package utils

type errorList struct {
	errors []error
}

func NewErrorList(errors []error) *errorList {
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

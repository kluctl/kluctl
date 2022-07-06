package yaml

import (
	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/reflectwalk"
	"reflect"
)

var (
	Validator = validator.New()
)

type structValidationWalker struct {
}

func (w *structValidationWalker) Struct(v reflect.Value) error {
	v2 := v.Interface()
	err := Validator.Struct(v2)
	if err != nil {
		return err
	}
	return nil
}

func (w *structValidationWalker) StructField(reflect.StructField, reflect.Value) error {
	return nil
}

func ValidateStructs(s interface{}) error {
	return reflectwalk.Walk(s, &structValidationWalker{})
}

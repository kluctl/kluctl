package jinja2

import (
	"github.com/hashicorp/go-multierror"
	"reflect"
)

type structStringCollectorEntry struct {
	s      string
	setter func(s string)
}

type structStringCollector struct {
	j       *Jinja2
	strings []structStringCollectorEntry
}

func (w *structStringCollector) extractTemplateString(v reflect.Value) (string, bool) {
	if v.IsZero() {
		return "", false
	}

	t := v.Type()
	if t.Kind() == reflect.Interface || t.Kind() == reflect.Pointer {
		v = v.Elem()
		t = v.Type()
	}
	if t.Kind() == reflect.String {
		s := v.String()
		if !isMaybeTemplateString(s) {
			return "", false
		}
		return v.String(), true
	}
	return "", false
}

func (w *structStringCollector) addString(s string, setter func(s string)) {
	w.strings = append(w.strings, structStringCollectorEntry{
		s:      s,
		setter: setter,
	})
}

func (w *structStringCollector) walkStruct(v reflect.Value) error {
	t := v.Type()
	l := t.NumField()

	for i := 0; i < l; i++ {
		f := v.Field(i)
		if s, ok := w.extractTemplateString(f); ok {
			w.addString(s, func(s string) {
				if f.Type().Kind() == reflect.Pointer {
					f.Set(reflect.ValueOf(&s))
				} else {
					f.Set(reflect.ValueOf(s))
				}
			})
		} else {
			err := w.walkValue(f)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *structStringCollector) walkList(v reflect.Value) error {
	l := v.Len()
	for i := 0; i < l; i++ {
		e := v.Index(i)
		if s, ok := w.extractTemplateString(e); ok {
			w.addString(s, func(s string) {
				if e.Type().Kind() == reflect.Pointer {
					e.Set(reflect.ValueOf(&s))
				} else {
					e.Set(reflect.ValueOf(s))
				}
			})
		} else {
			err := w.walkValue(e)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *structStringCollector) walkMap(v reflect.Value) error {
	it := v.MapRange()
	for it.Next() {
		ik := it.Key()
		iv := it.Value()

		if s, ok := w.extractTemplateString(iv); ok {
			if isMaybeTemplateString(s) {
				w.addString(s, func(s string) {
					if iv.Type().Kind() == reflect.Pointer {
						v.SetMapIndex(ik, reflect.ValueOf(&s))
					} else {
						v.SetMapIndex(ik, reflect.ValueOf(s))
					}
				})
			}
		} else {
			err := w.walkValue(iv)
			if err != nil {
				return err
			}
		}

		if s, ok := w.extractTemplateString(ik); ok {
			if isMaybeTemplateString(s) {
				w.addString(s, func(s string) {
					iv2 := v.MapIndex(ik)
					v.SetMapIndex(ik, reflect.ValueOf(nil))
					v.SetMapIndex(reflect.ValueOf(s), iv2)
				})
			}
		}
	}
	return nil
}

func (w *structStringCollector) walkValue(v reflect.Value) error {
	if v.IsZero() {
		return nil
	}

	v = reflect.Indirect(v)
	t := v.Type()

	switch t.Kind() {
	case reflect.Interface, reflect.Pointer:
		return w.walkValue(v.Elem())
	case reflect.Slice, reflect.Array:
		return w.walkList(v)
	case reflect.Struct:
		return w.walkStruct(v)
	case reflect.Map:
		return w.walkMap(v)
	}
	return nil
}

func (j *Jinja2) RenderStruct(o interface{}, opts ...Jinja2Opt) (bool, error) {
	w := &structStringCollector{j: j}
	v := reflect.ValueOf(o)
	err := w.walkValue(v)
	if err != nil {
		return false, err
	}

	var jobs []*RenderJob

	for _, sv := range w.strings {
		jobs = append(jobs, &RenderJob{Template: sv.s})
	}

	err = j.RenderStrings(jobs, opts...)
	if err != nil {
		return false, err
	}

	changed := false

	var retErr *multierror.Error
	for i, sv := range w.strings {
		job := jobs[i]
		if job.Error != nil {
			retErr = multierror.Append(retErr, job.Error)
		} else {
			if sv.s != *job.Result {
				sv.setter(*job.Result)
				changed = true
			}
		}
	}
	return changed, retErr.ErrorOrNil()
}

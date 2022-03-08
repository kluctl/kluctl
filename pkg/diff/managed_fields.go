package diff

import (
	"bytes"
	"fmt"
	"github.com/codablock/kluctl/pkg/utils/uo"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"regexp"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/value"
	"strconv"
)

type LostOwnership struct {
	Field   string
	Message string
}

var forceApplyFieldAnnotationRegex = regexp.MustCompile(`^kluctl.io/force-apply-field(-\d*)?$`)
var overwriteAllowedManagers = []*regexp.Regexp{
	regexp.MustCompile("kluctl"),
	regexp.MustCompile("kubectl"),
	regexp.MustCompile("kubectl-.*"),
	regexp.MustCompile("rancher"),
	regexp.MustCompile("k9s"),
}

func checkListItemMatch(o interface{}, pathElement fieldpath.PathElement, index int) (bool, error) {
	if pathElement.Key != nil {
		m, ok := o.(map[string]interface{})
		if !ok {
			return false, fmt.Errorf("object is not a map")
		}
		for _, f := range *pathElement.Key {
			c, ok := m[f.Name]
			if !ok {
				return false, nil
			}
			lhs := value.NewValueInterface(c)
			if value.Compare(lhs, f.Value) != 0 {
				return false, nil
			}
		}
		return true, nil
	} else if pathElement.Value != nil {
		lhs := value.NewValueInterface(o)
		if value.Compare(lhs, *pathElement.Value) == 0 {
			return true, nil
		}
		return false, nil
	} else if pathElement.Index != nil {
		if *pathElement.Index == index {
			return true, nil
		}
		return false, nil
	} else {
		return false, fmt.Errorf("unexpected path element")
	}
}

func convertToKeyList(remote *uo.UnstructuredObject, path fieldpath.Path) ([]interface{}, bool, error) {
	var ret []interface{}
	var o interface{} = remote.Object
	for _, e := range path {
		if e.FieldName != nil {
			ret = append(ret, *e.FieldName)
			x, found, err := uo.GetChild(o, *e.FieldName)
			if err != nil {
				return nil, false, err
			}
			if !found {
				return ret, false, nil
			}
			o = x
		} else {
			l, ok := o.([]interface{})
			if !ok {
				return nil, false, fmt.Errorf("object is not a list")
			}
			found := false
			for i, x := range l {
				match, err := checkListItemMatch(x, e, i)
				if err != nil {
					return nil, false, err
				}
				if match {
					found = true
					ret = append(ret, i)
					o = x
					break
				}
			}
			if !found {
				return ret, false, nil
			}
		}
	}
	return ret, true, nil
}

func ResolveFieldManagerConflicts(local *uo.UnstructuredObject, remote *uo.UnstructuredObject, conflictStatus metav1.Status) (*uo.UnstructuredObject, []LostOwnership, error) {
	managedFields := remote.GetK8sManagedFields()

	type managersByField struct {
		// "stupid" because the string representation of field pathes might be ambiguous as k8s does not escape dots
		stupidPath string
		pathes     []fieldpath.Path
		managers   []string
	}

	managersByFields := make(map[string]*managersByField)

	for _, mf := range managedFields {
		fieldSet := fieldpath.NewSet()
		err := fieldSet.FromJSON(bytes.NewReader(mf.FieldsV1.Raw))
		if err != nil {
			return nil, nil, err
		}

		fieldSet.Iterate(func(path fieldpath.Path) {
			s := path.String()
			if _, ok := managersByFields[s]; !ok {
				managersByFields[s] = &managersByField{stupidPath: s}
			}
			m, _ := managersByFields[s]
			found := false
			for _, p := range m.pathes {
				if p.Equals(path) {
					found = true
					break
				}
			}
			if !found {
				m.pathes = append(m.pathes, path)
			}
			m.managers = append(m.managers, mf.Manager)
		})
	}

	ret := local.Clone()

	forceApplyAll := false
	if x := local.GetK8sAnnotation("kluctl.io/force-apply"); x != nil {
		forceApplyAll, _ = strconv.ParseBool(*x)
	}

	forceApplyFields := make(map[string]bool)
	for k, v := range local.GetK8sAnnotations() {
		if !forceApplyFieldAnnotationRegex.MatchString(k) {
			continue
		}
		j, err := uo.NewMyJsonPath(v)
		if err != nil {
			return nil, nil, err
		}
		fields, err := j.ListMatchingFields(ret)
		if err != nil {
			return nil, nil, err
		}
		for _, f := range fields {
			forceApplyFields[uo.KeyListToJsonPath(f)] = true
		}
	}

	var lostOwnership []LostOwnership
	for _, cause := range conflictStatus.Details.Causes {
		if cause.Type != metav1.CauseTypeFieldManagerConflict {
			return nil, nil, fmt.Errorf("unknown type %s", cause.Type)
		}

		mf, ok := managersByFields[cause.Field]
		if !ok {
			return nil, nil, fmt.Errorf("could not find matching field for path '%s'", cause.Field)
		}
		if len(mf.pathes) != 1 {
			return nil, nil, fmt.Errorf("field path '%s' is ambiguous", cause.Field)
		}

		p, found, err := convertToKeyList(remote, mf.pathes[0])
		if err != nil {
			return nil, nil, err
		}
		if !found {
			return nil, nil, fmt.Errorf("field '%s' not found in remote object", cause.Field)
		}
		localValue, found, err := local.GetNestedField(p...)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			return nil, nil, fmt.Errorf("field '%s' not found in local object", cause.Field)
		}
		remoteValue, found, err := remote.GetNestedField(p...)
		if !found {
			log.Fatalf("field '%s' not found in remote object...which can't be!", cause.Field)
		}

		overwrite := true
		if !forceApplyAll {
			for _, mfn := range mf.managers {
				found := false
				for _, oa := range overwriteAllowedManagers {
					if oa.MatchString(mfn) {
						found = true
						break
					}
				}
				if !found {
					overwrite = false
					break
				}
			}
			if _, ok := forceApplyFields[uo.KeyListToJsonPath(p)]; ok {
				overwrite = true
			}
		}

		if !overwrite {
			j, err := uo.NewMyJsonPath(uo.KeyListToJsonPath(p))
			if err != nil {
				return nil, nil, err
			}
			err = j.Del(ret.Object)
			if err != nil {
				return nil, nil, err
			}

			if !reflect.DeepEqual(localValue, remoteValue) {
				lostOwnership = append(lostOwnership, LostOwnership{
					Field:   cause.Field,
					Message: cause.Message,
				})
			}
		}
	}

	return ret, lostOwnership, nil
}

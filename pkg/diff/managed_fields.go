package diff

import (
	"bytes"
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func convertToKeyList(remote *unstructured.Unstructured, path fieldpath.Path) ([]interface{}, bool, error) {
	var ret []interface{}
	var o interface{} = remote.Object
	for _, e := range path {
		if e.FieldName != nil {
			ret = append(ret, *e.FieldName)
			x, found, err := utils.GetChild(o, *e.FieldName)
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

func ResolveFieldManagerConflicts(local *unstructured.Unstructured, remote *unstructured.Unstructured, conflictStatus metav1.Status) (*unstructured.Unstructured, []LostOwnership, error) {
	managedFields := remote.GetManagedFields()

	// "stupid" because the string representation in "details.causes.field" might be ambiguous as k8s does not escape dots
	fieldsAsStupidStrings := make(map[string][]fieldpath.Path)
	managersByFields := make(map[string][]string)

	for _, mf := range managedFields {
		fieldSet := fieldpath.NewSet()
		err := fieldSet.FromJSON(bytes.NewReader(mf.FieldsV1.Raw))
		if err != nil {
			return nil, nil, err
		}

		fieldSet.Iterate(func(path fieldpath.Path) {
			s := path.String()
			fieldsAsStupidStrings[s] = append(fieldsAsStupidStrings[s], path)
			managersByFields[s] = append(managersByFields[s], mf.Manager)
		})
	}

	ret := utils.CopyUnstructured(local)

	forceApplyAll := false
	if x, ok := local.GetAnnotations()[`metadata.annotations["kluctl.io/force-apply"]`]; ok {
		forceApplyAll, _ = strconv.ParseBool(x)
	}

	forceApplyFields := make(map[string]bool)
	for k, v := range local.GetAnnotations() {
		if !forceApplyFieldAnnotationRegex.MatchString(k) {
			continue
		}
		j, err := utils.NewMyJsonPath(v)
		if err != nil {
			return nil, nil, err
		}
		fields, err := j.ListMatchingFields(ret.Object)
		if err != nil {
			return nil, nil, err
		}
		for _, f := range fields {
			forceApplyFields[utils.KeyListToJsonPath(f)] = true
		}
	}

	var lostOwnership []LostOwnership
	for _, cause := range conflictStatus.Details.Causes {
		if cause.Type != metav1.CauseTypeFieldManagerConflict {
			return nil, nil, fmt.Errorf("unknown type %s", cause.Type)
		}

		mfPath, ok := fieldsAsStupidStrings[cause.Field]
		if !ok {
			return nil, nil, fmt.Errorf("could not find matching field for path '%s'", cause.Field)
		}
		if len(mfPath) != 1 {
			return nil, nil, fmt.Errorf("field path '%s' is ambiguous", cause.Field)
		}

		p, found, err := convertToKeyList(remote, mfPath[0])
		if err != nil {
			return nil, nil, err
		}
		if !found {
			return nil, nil, fmt.Errorf("field '%s' not found in remote object", cause.Field)
		}
		localValue, found, err := utils.GetNestedChild(local.Object, p...)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			return nil, nil, fmt.Errorf("field '%s' not found in local object", cause.Field)
		}
		remoteValue, found, err := utils.GetNestedChild(remote.Object, p...)
		if !found {
			log.Fatalf("field '%s' not found in remote object...which can't be!", cause.Field)
		}

		overwrite := true
		if !forceApplyAll {
			mfbf, _ := managersByFields[mfPath[0].String()]
			for _, mfn := range mfbf {
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
			if _, ok := forceApplyFields[utils.KeyListToJsonPath(p)]; ok {
				overwrite = true
			}
		}

		if !overwrite {
			j, err := utils.NewMyJsonPath(utils.KeyListToJsonPath(p))
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

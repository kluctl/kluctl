package diff

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"regexp"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/value"
)

type LostOwnership struct {
	Field   string
	Message string
}

var forceApplyFieldAnnotationRegex = regexp.MustCompile(`^kluctl.io/force-apply-field(-\d*)?$`)
var forceApplyManagerAnnotationRegex = regexp.MustCompile(`^kluctl.io/force-apply-manager(-\d*)?$`)
var ignoreConflictsFieldAnnotationRegex = regexp.MustCompile(`^kluctl.io/ignore-conflicts-field(-\d*)?$`)
var ignoreConflictsManagerAnnotationRegex = regexp.MustCompile(`^kluctl.io/ignore-conflicts-manager(-\d*)?$`)

var overwriteAllowedManagers = []*regexp.Regexp{
	regexp.MustCompile("^kluctl$"),
	regexp.MustCompile("^kluctl-.*$"),
	regexp.MustCompile("^kubectl$"),
	regexp.MustCompile("^kubectl-.*$"),
	regexp.MustCompile("^rancher$"),
	regexp.MustCompile("^k9s$"),
}

type ConflictResolver struct {
	Configs []types.ConflictResolutionConfig
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

func convertToKeyList(remote *uo.UnstructuredObject, path fieldpath.Path) (uo.KeyPath, bool, error) {
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

type managersByField struct {
	pathes   []fieldpath.Path
	managers []string
}

// buildManagersByField returns a map indexed by field path and another map indexed by manager name
func (cr *ConflictResolver) buildManagersByField(remote *uo.UnstructuredObject) (map[string]*managersByField, map[string][]*managersByField, error) {
	managedFields := remote.GetK8sManagedFields()

	managersByFields := map[string]*managersByField{}
	fieldsByManagers := map[string][]*managersByField{}

	for _, mf := range managedFields {
		fields, ok, err := mf.GetNestedObject("fieldsV1")
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		fieldSet, _, err := convertManagedFields(fields.Object)
		if err != nil {
			return nil, nil, err
		}

		mgr, ok, err := mf.GetNestedString("manager")
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("manager field is missing")
		}

		fieldSet.Iterate(func(path fieldpath.Path) {
			path = path.Copy()
			s := path.String()
			if _, ok := managersByFields[s]; !ok {
				managersByFields[s] = &managersByField{}
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
			m.managers = append(m.managers, mgr)
			fieldsByManagers[mgr] = append(fieldsByManagers[mgr], m)
		})
	}

	return managersByFields, fieldsByManagers, nil
}

func (cr *ConflictResolver) buildConflictResolutionConfigs(local *uo.UnstructuredObject) []types.ConflictResolutionConfig {
	var ret []types.ConflictResolutionConfig
	ret = append(ret, cr.Configs...)

	forceApplyAll := local.GetK8sAnnotationBoolNoError("kluctl.io/force-apply", false)
	ignoreConflictsAll := local.GetK8sAnnotationBoolNoError("kluctl.io/ignore-conflicts", false)

	// order is important here. force-apply actions must come before ignore actions so that the ignore actions always
	// take precedence

	if forceApplyAll {
		ret = append(ret, types.ConflictResolutionConfig{
			FieldPath: []string{".."}, // match all fields
			Action:    types.ConflictResolutionForceApply,
		})
	}
	for _, v := range local.GetK8sAnnotationsWithRegex(forceApplyManagerAnnotationRegex) {
		ret = append(ret, types.ConflictResolutionConfig{
			Manager: []string{v},
			Action:  types.ConflictResolutionForceApply,
		})
	}
	for _, v := range local.GetK8sAnnotationsWithRegex(forceApplyFieldAnnotationRegex) {
		ret = append(ret, types.ConflictResolutionConfig{
			FieldPath: []string{v},
			Action:    types.ConflictResolutionForceApply,
		})
	}

	if ignoreConflictsAll {
		ret = append(ret, types.ConflictResolutionConfig{
			FieldPath: []string{".."}, // match all fields
			Action:    types.ConflictResolutionIgnore,
		})
	}
	for _, v := range local.GetK8sAnnotationsWithRegex(ignoreConflictsManagerAnnotationRegex) {
		ret = append(ret, types.ConflictResolutionConfig{
			Manager: []string{v},
			Action:  types.ConflictResolutionIgnore,
		})
	}
	for _, v := range local.GetK8sAnnotationsWithRegex(ignoreConflictsFieldAnnotationRegex) {
		ret = append(ret, types.ConflictResolutionConfig{
			FieldPath: []string{v},
			Action:    types.ConflictResolutionIgnore,
		})
	}

	return ret
}

func (cr *ConflictResolver) collectFields(local *uo.UnstructuredObject, remote *uo.UnstructuredObject, fieldsByManager map[string][]*managersByField, configs []types.ConflictResolutionConfig) (map[string]types.ConflictResolutionAction, error) {
	result := map[string]types.ConflictResolutionAction{}

	checkMatch := func(v string, m *string) bool {
		if v == "" || m == nil {
			return true
		}
		return v == *m
	}

	var allFields map[string]bool
	initAllFields := func() error {
		if allFields != nil {
			return nil
		}
		allFields = map[string]bool{}

		jp := uo.NewMyJsonPathMust("..")
		l, err := jp.ListMatchingFields(local)
		if err != nil {
			return err
		}
		for _, x := range l {
			allFields[x.ToJsonPath()] = true
		}
		l, err = jp.ListMatchingFields(local)
		if err != nil {
			return err
		}
		for _, x := range l {
			allFields[x.ToJsonPath()] = true
		}
		return nil
	}

	ref := local.GetK8sRef()

	for _, cfg := range configs {
		if !checkMatch(ref.Group, cfg.Group) {
			continue
		}
		if !checkMatch(ref.Kind, cfg.Kind) {
			continue
		}
		if !checkMatch(ref.Namespace, cfg.Namespace) {
			continue
		}
		if !checkMatch(ref.Name, cfg.Name) {
			continue
		}

		for _, fp := range cfg.FieldPath {
			jp, err := uo.NewMyJsonPath(fp)
			if err != nil {
				return nil, err
			}

			fields, err := jp.ListMatchingFields(local)
			if err != nil {
				return nil, err
			}
			for _, f := range fields {
				result[f.ToJsonPath()] = cfg.Action
			}

			fields, err = jp.ListMatchingFields(remote)
			if err != nil {
				return nil, err
			}
			for _, f := range fields {
				result[f.ToJsonPath()] = cfg.Action
			}
		}

		for _, fp := range cfg.FieldPathRegex {
			rx, err := regexp.Compile(fp)
			if err != nil {
				return nil, err
			}
			err = initAllFields()
			if err != nil {
				return nil, err
			}
			for f, _ := range allFields {
				if rx.MatchString(f) {
					result[f] = cfg.Action
				}
			}
		}

		for _, m := range cfg.Manager {
			rx, err := regexp.Compile(m)
			if err != nil {
				return nil, err
			}
			for mgrName, mfs := range fieldsByManager {
				if rx.MatchString(mgrName) {
					for _, mf := range mfs {
						for _, p := range mf.pathes {
							kl, found, err := convertToKeyList(remote, p)
							if err != nil {
								return nil, err
							}
							if found {
								result[kl.ToJsonPath()] = cfg.Action
							}
						}
					}
				}
			}
		}
	}
	return result, nil
}

func (cr *ConflictResolver) ResolveConflicts(local *uo.UnstructuredObject, remote *uo.UnstructuredObject, conflictStatus metav1.Status) (*uo.UnstructuredObject, []LostOwnership, error) {
	managersByFields, fieldsByManager, err := cr.buildManagersByField(remote)
	if err != nil {
		return nil, nil, err
	}

	resolutionConfigs := cr.buildConflictResolutionConfigs(local)
	resolutionFields, err := cr.collectFields(local, remote, fieldsByManager, resolutionConfigs)
	if err != nil {
		return nil, nil, err
	}

	ret := local.Clone()
	var lostOwnership []LostOwnership
	for _, cause := range conflictStatus.Details.Causes {
		if cause.Type != metav1.CauseTypeFieldManagerConflict {
			return nil, nil, fmt.Errorf("unknown type %s", cause.Type)
		}

		// TODO fields are ambiguous at this point because the apiserver serializes fieldpath.Path as a string
		// this causes maps with keys that allow dots to be possibly ambiguous.
		// Not sure what we should do about this.
		mf, ok := managersByFields[cause.Field]
		if !ok {
			return nil, nil, fmt.Errorf("%s. Could not find matching field for path '%s'", cause.Message, cause.Field)
		}
		if len(mf.pathes) != 1 {
			return nil, nil, fmt.Errorf("%s. Field path '%s' is ambiguous", cause.Message, cause.Field)
		}

		localKeyPath, found, err := convertToKeyList(local, mf.pathes[0])
		if err != nil {
			return nil, nil, err
		}
		if !found {
			return nil, nil, fmt.Errorf("%s. Field '%s' not found in local object", cause.Message, cause.Field)
		}

		remoteKeyPath, found, err := convertToKeyList(remote, mf.pathes[0])
		if err != nil {
			return nil, nil, err
		}
		if !found {
			return nil, nil, fmt.Errorf("%s. Field '%s' not found in remote object", cause.Message, cause.Field)
		}

		localValue, found, err := local.GetNestedField(localKeyPath...)
		if !found {
			panic(fmt.Sprintf("field '%s' not found in local object...which can't be!", cause.Field))
		}

		remoteValue, found, err := remote.GetNestedField(remoteKeyPath...)
		if !found {
			panic(fmt.Sprintf("field '%s' not found in remote object...which can't be!", cause.Field))
		}

		ignoreConflict := false
		overwrite := false

		action, ok := resolutionFields[localKeyPath.ToJsonPath()]
		if !ok {
			action, ok = resolutionFields[remoteKeyPath.ToJsonPath()]
		}
		if ok {
			ignoreConflict = action == types.ConflictResolutionIgnore
			overwrite = action == types.ConflictResolutionForceApply
		}

		if !ignoreConflict {
			// only force-apply known user facing editors when not explicitly ignored
			for _, mfn := range mf.managers {
				found := false
				for _, oa := range overwriteAllowedManagers {
					if oa.MatchString(mfn) {
						found = true
						break
					}
				}
				if found {
					overwrite = true
					break
				}
			}
		}

		if !overwrite {
			j, err := uo.NewMyJsonPath(localKeyPath.ToJsonPath())
			if err != nil {
				return nil, nil, err
			}
			err = j.Del(ret)
			if err != nil {
				return nil, nil, err
			}

			if !reflect.DeepEqual(localValue, remoteValue) && !ignoreConflict {
				lostOwnership = append(lostOwnership, LostOwnership{
					Field:   cause.Field,
					Message: cause.Message,
				})
			}
		}
	}

	return ret, lostOwnership, nil
}

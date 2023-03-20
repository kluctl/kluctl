package diff

import (
	"fmt"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	diff2 "github.com/r3labs/diff/v2"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

var notPresent = struct{}{}

func convertPath(path []string, o interface{}) (string, error) {
	var ret uo.KeyPath
	for _, p := range path {
		if i, err := strconv.ParseInt(p, 10, 32); err == nil {
			x, found, _ := uo.GetChild(o, int(i))
			if found {
				ret = append(ret, int(i))
				o = x
				continue
			}
		}
		x, found, err := uo.GetChild(o, p)
		if !found {
			return "", fmt.Errorf("path element %v is invalid: %w", p, err)
		}
		ret = append(ret, p)
		o = x
	}
	return ret.ToJsonPath(), nil
}

func Diff(oldObject *uo.UnstructuredObject, newObject *uo.UnstructuredObject) ([]result.Change, error) {
	differ, err := diff2.NewDiffer(diff2.AllowTypeMismatch(true))
	if err != nil {
		return nil, err
	}
	cl, err := differ.Diff(oldObject.Object, newObject.Object)
	if err != nil {
		return nil, err
	}

	var changes []result.Change
	for _, c := range cl {
		c2, err := convertChange(c, oldObject, newObject)
		if err != nil {
			return nil, err
		}
		err = updateUnifiedDiff(c2)
		if err != nil {
			return nil, err
		}
		changes = append(changes, *c2)
	}

	// The result of the above diff call is not stable
	stableSortChanges(changes)
	return changes, nil
}

func convertChange(c diff2.Change, oldObject *uo.UnstructuredObject, newObject *uo.UnstructuredObject) (*result.Change, error) {
	switch c.Type {
	case "create":
		p, err := convertPath(c.Path, newObject.Object)
		if err != nil {
			return nil, err
		}
		return &result.Change{
			Type:     "insert",
			JsonPath: p,
			NewValue: c.To,
		}, nil
	case "delete":
		p, err := convertPath(c.Path, oldObject.Object)
		if err != nil {
			return nil, err
		}
		return &result.Change{
			Type:     "delete",
			JsonPath: p,
			OldValue: c.From,
		}, nil
	case "update":
		p, err := convertPath(c.Path, newObject.Object)
		if err != nil {
			return nil, err
		}
		return &result.Change{
			Type:     "update",
			JsonPath: p,
			NewValue: c.To,
			OldValue: c.From,
		}, nil
	}
	return nil, fmt.Errorf("unknown change type %s", c.Type)
}

func updateUnifiedDiff(change *result.Change) error {
	switch change.Type {
	case "insert":
		ud, err := buildUnifiedDiff(notPresent, change.NewValue, false)
		if err != nil {
			return err
		}
		change.UnifiedDiff = ud
	case "delete":
		ud, err := buildUnifiedDiff(change.OldValue, notPresent, false)
		if err != nil {
			return err
		}
		change.UnifiedDiff = ud
	case "update":
		showType := false
		if reflect.TypeOf(change.OldValue) != reflect.TypeOf(change.NewValue) {
			showType = true
		}
		ud, err := buildUnifiedDiff(change.OldValue, change.NewValue, showType)
		if err != nil {
			return err
		}
		change.UnifiedDiff = ud
	default:
		return fmt.Errorf("unknown change type %s", change.Type)
	}
	return nil
}

func stableSortChanges(changes []result.Change) {
	changesStrs := make([]string, len(changes))
	changesIndexes := make([]int, len(changes))
	for i, _ := range changes {
		y, err := yaml.WriteYamlString(changes[i])
		if err != nil {
			panic(err)
		}
		changesStrs[i] = y
		changesIndexes[i] = i
	}

	sort.SliceStable(changesIndexes, func(i, j int) bool {
		return changesStrs[changesIndexes[i]] < changesStrs[changesIndexes[j]]
	})

	changesSorted := make([]result.Change, len(changes))
	for i, _ := range changes {
		changesSorted[i] = changes[changesIndexes[i]]
	}
	copy(changes, changesSorted)
}

func buildUnifiedDiff(a interface{}, b interface{}, showType bool) (string, error) {
	aStr, err := objectToDiffableString(a, showType)
	if err != nil {
		return "", err
	}
	bStr, err := objectToDiffableString(b, showType)
	if err != nil {
		return "", err
	}

	if len(aStr) == 0 {
		return prependStrToLines(bStr, "+"), nil
	} else if len(bStr) == 0 {
		return prependStrToLines(aStr, "-"), nil
	} else if strings.Index(aStr, "\n") == -1 && strings.Index(bStr, "\n") == -1 {
		return fmt.Sprintf("-%s\n+%s", aStr, bStr), nil
	}

	edits := myers.ComputeEdits(span.URIFromPath("a"), aStr, bStr)
	diff := fmt.Sprint(gotextdiff.ToUnified("a", "b", aStr, edits))
	// Skip diff header
	lines := strings.Split(diff, "\n")
	lines = lines[2:]
	return strings.Join(lines, "\n"), nil
}

func objectToDiffableString(o interface{}, showType bool) (string, error) {
	s, err := objectToDiffableStringNoType(o)
	if err != nil {
		return "", err
	}
	if showType {
		t := "<nil>"
		if o != nil {
			t = reflect.TypeOf(o).Name()
		}
		s += fmt.Sprintf(" (type: %s)", t)
	}
	return s, nil
}

func objectToDiffableStringNoType(o interface{}) (string, error) {
	if o == nil {
		return "<nil>", nil
	}
	if o == notPresent {
		return "", nil
	}
	if v, ok := o.(string); ok {
		return v, nil
	}

	isYaml := false
	if _, ok := o.(map[string]interface{}); ok {
		isYaml = true
	} else if _, ok := o.([]interface{}); ok {
		isYaml = true
	}

	if isYaml {
		b, err := yaml.WriteYamlString(o)
		if err != nil {
			return "", err
		}
		return b, nil
	} else {
		return fmt.Sprint(o), nil
	}
}

func prependStrToLines(s string, prepend string) string {
	if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}

	lines := strings.Split(s, "\n")
	for i, _ := range lines {
		lines[i] = prepend + lines[i]
	}
	return strings.Join(lines, "\n")
}

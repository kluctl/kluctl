package diff

import (
	"fmt"
	"github.com/kluctl/kluctl/pkg/types"
	"github.com/kluctl/kluctl/pkg/utils/uo"
	"github.com/kluctl/kluctl/pkg/yaml"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	diff2 "github.com/r3labs/diff/v2"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

var notPresent = struct{}{}

func convertPath(path []string, o interface{}) (string, error) {
	var ret []interface{}
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
	return uo.KeyListToJsonPath(ret), nil
}

func Diff(oldObject *uo.UnstructuredObject, newObject *uo.UnstructuredObject) ([]types.Change, error) {
	differ, err := diff2.NewDiffer(diff2.AllowTypeMismatch(true))
	if err != nil {
		return nil, err
	}
	cl, err := differ.Diff(oldObject.Object, newObject.Object)
	if err != nil {
		return nil, err
	}

	var changes []types.Change
	for _, c := range cl {
		c2, err := convertChange(c, oldObject, newObject)
		if err != nil {
			return nil, err
		}
		changes = append(changes, *c2)
	}

	// The result of the above diff call is not stable
	stableSortChanges(changes)
	return changes, nil
}

func convertChange(c diff2.Change, oldObject *uo.UnstructuredObject, newObject *uo.UnstructuredObject) (*types.Change, error) {
	switch c.Type {
	case "create":
		ud, err := buildUnifiedDiff(notPresent, c.To, false)
		if err != nil {
			return nil, err
		}
		p, err := convertPath(c.Path, newObject.Object)
		if err != nil {
			return nil, err
		}
		return &types.Change{
			Type:        "insert",
			JsonPath:    p,
			NewValue:    c.To,
			UnifiedDiff: ud,
		}, nil
	case "delete":
		ud, err := buildUnifiedDiff(c.From, notPresent, false)
		if err != nil {
			return nil, err
		}
		p, err := convertPath(c.Path, oldObject.Object)
		if err != nil {
			return nil, err
		}
		return &types.Change{
			Type:        "delete",
			JsonPath:    p,
			OldValue:    c.From,
			UnifiedDiff: ud,
		}, nil
	case "update":
		showType := false
		if reflect.TypeOf(c.From) != reflect.TypeOf(c.To) {
			showType = true
		}
		ud, err := buildUnifiedDiff(c.From, c.To, showType)
		if err != nil {
			return nil, err
		}
		p, err := convertPath(c.Path, newObject.Object)
		if err != nil {
			return nil, err
		}
		return &types.Change{
			Type:        "update",
			JsonPath:    p,
			NewValue:    c.To,
			OldValue:    c.From,
			UnifiedDiff: ud,
		}, nil
	}
	return nil, fmt.Errorf("unknown change type %s", c.Type)
}

func stableSortChanges(changes []types.Change) {
	changesStrs := make([]string, len(changes))
	changesIndexes := make([]int, len(changes))
	for i, _ := range changes {
		y, err := yaml.WriteYamlString(changes[i])
		if err != nil {
			log.Panic(err)
		}
		changesStrs[i] = y
		changesIndexes[i] = i
	}

	sort.SliceStable(changesIndexes, func(i, j int) bool {
		return changesStrs[changesIndexes[i]] < changesStrs[changesIndexes[j]]
	})

	changesSorted := make([]types.Change, len(changes))
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
		s += fmt.Sprintf(" (type: %s)", reflect.TypeOf(o).Name())
	}
	return s, nil
}

func objectToDiffableStringNoType(o interface{}) (string, error) {
	if o == nil {
		return "null", nil
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

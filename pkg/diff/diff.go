package diff

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	diff2 "github.com/r3labs/diff/v2"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"strconv"
	"strings"
)

var notPresent = struct{}{}

func convertPath(path []string, o interface{}) (string, error) {
	var ret []interface{}
	for _, p := range path {
		if i, err := strconv.ParseInt(p, 10, 32); err == nil {
			x, found, _ := utils.GetChild(o, int(i))
			if found {
				ret = append(ret, int(i))
				o = x
				continue
			}
		}
		x, found, err := utils.GetChild(o, p)
		if !found {
			return "", fmt.Errorf("path element %v is invalid: %w", p, err)
		}
		ret = append(ret, p)
		o = x
	}
	return utils.KeyListToJsonPath(ret), nil
}

func Diff(oldObject *unstructured.Unstructured, newObject *unstructured.Unstructured) ([]types.Change, error) {
	cl, err := diff2.Diff(oldObject.Object, newObject.Object)
	if err != nil {
		return nil, err
	}

	var changes []types.Change
	for _, c := range cl {
		switch c.Type {
		case "create":
			ud, err := buildUnifiedDiff(notPresent, c.To)
			if err != nil {
				return nil, err
			}
			p, err := convertPath(c.Path, newObject.Object)
			if err != nil {
				return nil, err
			}
			changes = append(changes, types.Change{
				Type:        "insert",
				JsonPath:    p,
				NewValue:    c.To,
				UnifiedDiff: ud,
			})
		case "delete":
			ud, err := buildUnifiedDiff(c.From, notPresent)
			if err != nil {
				return nil, err
			}
			p, err := convertPath(c.Path, oldObject.Object)
			if err != nil {
				return nil, err
			}
			changes = append(changes, types.Change{
				Type:        "delete",
				JsonPath:    p,
				OldValue:    c.From,
				UnifiedDiff: ud,
			})
		case "update":
			ud, err := buildUnifiedDiff(c.From, c.To)
			if err != nil {
				return nil, err
			}
			p, err := convertPath(c.Path, newObject.Object)
			if err != nil {
				return nil, err
			}
			changes = append(changes, types.Change{
				Type:        "update",
				JsonPath:    p,
				NewValue:    c.To,
				OldValue:    c.From,
				UnifiedDiff: ud,
			})
		}
	}
	return changes, nil
}

func buildUnifiedDiff(a interface{}, b interface{}) (string, error) {
	aStr, err := objectToDiffableString(a)
	if err != nil {
		return "", err
	}
	bStr, err := objectToDiffableString(b)
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

func objectToDiffableString(o interface{}) (string, error) {
	if o == nil {
		return "null", nil
	}
	if o == notPresent {
		return "", nil
	}
	if v, ok := o.(string); ok {
		return v, nil
	}

	b, err := yaml.Marshal(o)
	if err != nil {
		return "", err
	}
	return string(b), nil
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

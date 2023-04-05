package result

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/sergi/go-diff/diffmatchpatch"
	"strings"
	"sync"
)

type CompactedObjects []ResultObject

type CompactedObject struct {
	BaseObject

	Rendered string `json:"rendered,omitempty"`
	Remote   string `json:"remote,omitempty"`
	Applied  string `json:"applied,omitempty"`
}

func (l CompactedObjects) MarshalJSON() ([]byte, error) {
	compactedList := make([]CompactedObject, len(l))

	d := diffmatchpatch.New()

	createPatchOrFull := func(prevJson *string, o *uo.UnstructuredObject) string {
		if o == nil {
			return ""
		}
		j, err := yaml.WriteJsonString(o)
		if err != nil {
			// we are a point where this was parsed/written so many times, it really shouldn't error
			panic(err)
		}
		if *prevJson == "" {
			*prevJson = j
			return "full: " + j
		}

		diff := d.DiffMain(*prevJson, j, false)
		delta := d.DiffToDelta(diff)
		*prevJson = j

		if len(delta) < len(j) {
			return "delta: " + delta
		} else {
			return "full: " + j
		}
	}

	var wg sync.WaitGroup
	for i, o := range l {
		i := i
		o := o
		wg.Add(1)
		go func() {
			defer wg.Done()
			var prevJson string
			compactedList[i].BaseObject = o.BaseObject
			compactedList[i].Rendered = createPatchOrFull(&prevJson, o.Rendered)
			compactedList[i].Remote = createPatchOrFull(&prevJson, o.Remote)
			compactedList[i].Applied = createPatchOrFull(&prevJson, o.Applied)
		}()
	}
	wg.Wait()

	return json.Marshal(compactedList)
}

func (l *CompactedObjects) UnmarshalJSON(b []byte) error {
	var compactedList []CompactedObject
	err := json.Unmarshal(b, &compactedList)
	if err != nil {
		return err
	}

	d := diffmatchpatch.New()

	patchAndUnmarshal := func(prevJson *string, s string) (*uo.UnstructuredObject, error) {
		if s == "" {
			return nil, nil
		}

		if strings.HasPrefix(s, "full: ") {
			full := s[6:]
			*prevJson = full
			return uo.FromString(full)
		} else if strings.HasPrefix(s, "delta: ") {
			if *prevJson == "" {
				return nil, fmt.Errorf("prevJson empty")
			}
			delta := s[7:]
			diff, err := d.DiffFromDelta(*prevJson, delta)
			if err != nil {
				return nil, err
			}
			patch := d.PatchMake(diff)
			newJson, result := d.PatchApply(patch, *prevJson)
			for _, b := range result {
				if !b {
					return nil, fmt.Errorf("patch did not fully apply")
				}
			}
			o, err := uo.FromString(newJson)
			if err != nil {
				return nil, err
			}
			*prevJson = newJson
			return o, nil
		} else {
			return nil, fmt.Errorf("unexpected object/delta")
		}
	}

	ret := make([]ResultObject, len(compactedList))

	gh := utils.NewGoHelper(context.Background(), -1)
	for i, o := range compactedList {
		i := i
		o := o
		gh.RunE(func() error {
			var err error

			o2 := ResultObject{}
			o2.BaseObject = o.BaseObject

			prevJson := ""
			o2.Rendered, err = patchAndUnmarshal(&prevJson, o.Rendered)
			if err != nil {
				return err
			}
			o2.Remote, err = patchAndUnmarshal(&prevJson, o.Remote)
			if err != nil {
				return err
			}
			o2.Applied, err = patchAndUnmarshal(&prevJson, o.Applied)
			if err != nil {
				return err
			}
			ret[i] = o2
			return nil
		})
	}
	gh.Wait()
	err = gh.ErrorOrNil()
	if err != nil {
		return err
	}
	*l = ret
	return nil
}

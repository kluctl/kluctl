package test_resources

import (
	"context"
	"embed"
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/validation"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

//go:embed *.yaml
var Yamls embed.FS

func GetYamlTmpFile(name string) string {
	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	tmpFile.Close()

	b, err := Yamls.ReadFile(name)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(tmpFile.Name(), b, 0o600)
	if err != nil {
		panic(err)
	}

	return tmpFile.Name()
}

// poor mans resource mapper :)
func guessGVR(gvk schema.GroupVersionKind) schema.GroupVersionResource {
	gvr := schema.GroupVersionResource{
		Group:   gvk.Group,
		Version: gvk.Version,
	}
	if strings.HasSuffix(gvk.Kind, "y") {
		gvr.Resource = strings.ToLower(gvk.Kind)[:len(gvk.Kind)-1] + "ies"
	} else {
		gvr.Resource = strings.ToLower(gvk.Kind) + "s"
	}
	return gvr
}

func prio(x *uo.UnstructuredObject) int {
	switch x.GetK8sGVK().Kind {
	case "Namespace":
		return 100
	case "CustomResourceDefinition":
		return 100
	default:
		return 0
	}
}

func waitReadiness(k *test_utils.EnvTestCluster, x *uo.UnstructuredObject) {
	for true {
		u, err := k.DynamicClient.Resource(guessGVR(x.GetK8sGVK())).Get(context.Background(), x.GetK8sName(), metav1.GetOptions{})
		if err != nil {
			panic(err)
		}
		vr := validation.ValidateObject(nil, uo.FromUnstructured(u), true, true)
		if vr.Ready {
			break
		} else {
			time.Sleep(time.Millisecond * 100)
		}
	}
}

func ApplyYaml(t *testing.T, name string, k *test_utils.EnvTestCluster) {
	tmpFile := GetYamlTmpFile(name)
	defer os.Remove(tmpFile)

	doPanic := func(err error) {
		if t != nil {
			t.Fatal(err)
		} else {
			panic(err)
		}
	}

	docs, err := yaml.ReadYamlAllFile(tmpFile)
	if err != nil {
		doPanic(err)
	}

	var objects []*uo.UnstructuredObject
	for _, doc := range docs {
		m, ok := doc.(map[string]any)
		if !ok {
			doPanic(fmt.Errorf("not a map!"))
		}
		x := uo.FromMap(m)
		objects = append(objects, x)
	}

	sort.SliceStable(objects, func(i, j int) bool {
		return prio(objects[i]) > prio(objects[j])
	})

	var wg sync.WaitGroup
	prevPrio := prio(objects[0])
	for _, x := range objects {
		x := x

		p := prio(x)
		if p != prevPrio {
			wg.Wait()
		}
		prevPrio = p

		wg.Add(1)
		go func() {
			defer wg.Done()

			data, err := yaml.WriteYamlBytes(x)
			if err != nil {
				doPanic(err)
			}

			gvr := guessGVR(x.GetK8sGVK())
			_, err = k.DynamicClient.Resource(gvr).
				Namespace(x.GetK8sNamespace()).
				Patch(context.Background(), x.GetK8sName(), types.ApplyPatchType, data, metav1.PatchOptions{
					FieldManager: "e2e-tests",
				})
			if err != nil {
				doPanic(err)
			}

			// wait for CRDs to get accepted
			if x.GetK8sGVK().Kind == "CustomResourceDefinition" {
				waitReadiness(k, x)
				// add some safety net...for some reason the envtest api server still fails if not waiting
				time.Sleep(200 * time.Millisecond)
			}
		}()
	}
	wg.Wait()
}

package test_resources

import (
	"context"
	"embed"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/validation"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"strings"
	"time"

	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils"
)

//go:embed *.yaml
var Yamls embed.FS

func GetYamlTmpFile(name string) string {
	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	tmpFile.Close()

	err = utils.FsCopyFile(Yamls, name, tmpFile.Name())
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

func ApplyYaml(name string, k *test_utils.EnvTestCluster) {
	tmpFile := GetYamlTmpFile(name)
	defer os.Remove(tmpFile)

	docs, err := yaml.ReadYamlAllFile(tmpFile)
	if err != nil {
		panic(err)
	}

	for _, doc := range docs {
		m, ok := doc.(map[string]any)
		if !ok {
			panic("not a map!")
		}
		x := uo.FromMap(m)

		data, err := yaml.WriteYamlBytes(x)
		if err != nil {
			panic(err)
		}

		gvr := guessGVR(x.GetK8sGVK())
		_, err = k.DynamicClient.Resource(gvr).
			Namespace(x.GetK8sNamespace()).
			Patch(context.Background(), x.GetK8sName(), types.ApplyPatchType, data, metav1.PatchOptions{
				FieldManager: "e2e-tests",
			})
		if err != nil {
			panic(err)
		}

		// wait for CRDs to get accepted
		if x.GetK8sGVK().Kind == "CustomResourceDefinition" {
			for true {
				u, err := k.DynamicClient.Resource(gvr).Get(context.Background(), x.GetK8sName(), metav1.GetOptions{})
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
	}
}

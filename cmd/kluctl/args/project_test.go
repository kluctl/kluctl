package args

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	agekey "filippo.io/age"
	"github.com/getsops/sops/v3/age"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/sops_test_resources"
	"github.com/stretchr/testify/assert"
)

func TestArgsFromFileSops(t *testing.T) {
	key, err := sops_test_resources.TestResources.ReadFile("test-key.txt")
	assert.NoError(t, err)
	t.Setenv(age.SopsAgeKeyEnv, string(key))

	encrypted, err := sops_test_resources.TestResources.ReadFile("test.yaml")
	assert.NoError(t, err)

	for _, name := range []string{"args.yaml", "args.yml", "args.json", "args"} {
		t.Run(name, func(t *testing.T) {
			content := encrypted
			if filepath.Ext(name) == ".json" {
				o, err := uo.FromString(string(encrypted))
				assert.NoError(t, err)
				content, err = json.Marshal(o.Object)
				assert.NoError(t, err)
			}
			path := filepath.Join(t.TempDir(), name)
			assert.NoError(t, os.WriteFile(path, content, 0o600))

			flags := ArgsFlags{ArgsFromFile: []string{path}}
			args, err := flags.LoadArgs()
			assert.NoError(t, err)
			assert.Equal(t, uo.FromMap(map[string]any{"test1": map[string]any{"test2": float64(42)}}), args)

			unchanged, err := os.ReadFile(path)
			assert.NoError(t, err)
			assert.Equal(t, content, unchanged)

			// Later files retain their existing precedence, including over -a.
			plainPath := filepath.Join(t.TempDir(), "plain.yaml")
			assert.NoError(t, os.WriteFile(plainPath, []byte("test1:\n  test2: 43\nplain: true\n"), 0o600))
			flags.Arg = []string{"test1.test2=1", "cli=true"}
			flags.ArgsFromFile = []string{plainPath, path}
			args, err = flags.LoadArgs()
			assert.NoError(t, err)
			assert.Equal(t, uo.FromMap(map[string]any{
				"test1": map[string]any{"test2": float64(42)}, "plain": true, "cli": true,
			}), args)

			flags.ArgsFromFile = []string{path, plainPath}
			args, err = flags.LoadArgs()
			assert.NoError(t, err)
			assert.Equal(t, uo.FromMap(map[string]any{
				"test1": map[string]any{"test2": float64(43)}, "plain": true, "cli": true,
			}), args)
		})
	}
}

func TestArgsFromFileSopsDecryptionError(t *testing.T) {
	// A valid but unrelated key makes this independent of the user's keyring.
	identity, err := agekey.GenerateX25519Identity()
	assert.NoError(t, err)
	t.Setenv(age.SopsAgeKeyEnv, identity.String())
	t.Setenv(age.SopsAgeKeyFileEnv, filepath.Join(t.TempDir(), "missing-key.txt"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	encrypted, err := sops_test_resources.TestResources.ReadFile("test.yaml")
	assert.NoError(t, err)
	path := filepath.Join(t.TempDir(), "args.yaml")
	assert.NoError(t, os.WriteFile(path, encrypted, 0o600))

	flags := ArgsFlags{ArgsFromFile: []string{path}}
	args, err := flags.LoadArgs()
	assert.ErrorContains(t, err, "failed to decrypt args file "+path)
	assert.Nil(t, args)
}

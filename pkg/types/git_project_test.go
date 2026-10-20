package types

import (
	"encoding/json"
	"github.com/kluctl/kluctl/lib/git/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMarshalGitRef(t *testing.T) {
	s := `{"branch": "branch1"}`
	var ref types.GitRef
	err := json.Unmarshal([]byte(s), &ref)
	assert.NoError(t, err)
	assert.Equal(t, "branch1", ref.Branch)
	assert.Empty(t, ref.Tag)

	s = `{"tag": "tag1"}`
	ref = types.GitRef{}
	err = json.Unmarshal([]byte(s), &ref)
	assert.NoError(t, err)
	assert.Equal(t, "tag1", ref.Tag)
	assert.Empty(t, ref.Branch)
}

func TestMarshalGitRefErrors(t *testing.T) {
	err := json.Unmarshal([]byte(`{"branch": "branch1", "tag": "tag1"}`), &types.GitRef{})
	assert.EqualError(t, err, "only one of the ref fields can be set")

	err = json.Unmarshal([]byte(`{}`), &types.GitRef{})
	assert.EqualError(t, err, "either branch, tag, commit or ref must be set")
}

func TestMarshalGitRefRef(t *testing.T) {
	var ref types.GitRef
	err := json.Unmarshal([]byte(`{"ref": "refs/merge-requests/123/merge"}`), &ref)
	assert.NoError(t, err)
	assert.Equal(t, types.GitRef{Ref: "refs/merge-requests/123/merge"}, ref)
	assert.Equal(t, "refs/merge-requests/123/merge", ref.String())

	b, err := json.Marshal(ref)
	assert.NoError(t, err)
	assert.Equal(t, `{"ref":"refs/merge-requests/123/merge"}`, string(b))

	parsed, err := types.ParseGitRef("refs/pull/5/head")
	assert.NoError(t, err)
	assert.Equal(t, types.GitRef{Ref: "refs/pull/5/head"}, parsed)

	err = json.Unmarshal([]byte(`{"ref": "merge-requests/123/merge"}`), &types.GitRef{})
	assert.EqualError(t, err, "ref must be a full ref name starting with 'refs/'")
	err = json.Unmarshal([]byte(`{"ref": "refs/x", "branch": "b"}`), &types.GitRef{})
	assert.EqualError(t, err, "only one of the ref fields can be set")
}

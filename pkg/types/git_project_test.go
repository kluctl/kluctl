package types

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMarshaGitRefString(t *testing.T) {
	var ref GitRef
	err := json.Unmarshal([]byte(`"as_string"`), &ref)
	assert.NoError(t, err)
	assert.Equal(t, "as_string", ref.Ref)

	b, err := json.Marshal(&ref)
	assert.NoError(t, err)
	assert.Equal(t, `"as_string"`, string(b))
}

func TestMarshalGitRef(t *testing.T) {
	s := `{"branch": "branch1"}`
	var ref GitRef
	err := json.Unmarshal([]byte(s), &ref)
	assert.NoError(t, err)
	assert.Equal(t, "branch1", ref.Branch)
	assert.Empty(t, ref.Tag)

	s = `{"tag": "tag1"}`
	ref = GitRef{}
	err = json.Unmarshal([]byte(s), &ref)
	assert.NoError(t, err)
	assert.Equal(t, "tag1", ref.Tag)
	assert.Empty(t, ref.Branch)
}

func TestMarshalGitRefErrors(t *testing.T) {
	err := json.Unmarshal([]byte(`{"branch": "branch1", "tag": "tag1"}`), &GitRef{})
	assert.EqualError(t, err, "only one of the ref fields can be set")

	err = json.Unmarshal([]byte(`{}`), &GitRef{})
	assert.EqualError(t, err, "either branch or tag must be set")
}

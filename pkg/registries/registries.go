package registries

import (
	"github.com/google/go-containerregistry/pkg/crane"
)

func ListImageTags(image string) ([]string, error) {
	tags, err := crane.ListTags(image)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

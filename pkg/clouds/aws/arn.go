package aws

import (
	"strings"
)

type Resource struct {
	ResourceId   string
	ResourceType string
}

func SplitResource(resource string) (Resource, error) {
	result := Resource{resource, ""}
	if strings.Index(resource, "/") != -1 {
		s := strings.SplitN(resource, "/", 2)
		result.ResourceType = s[0]
		result.ResourceId = s[1]
	} else if strings.Index(resource, ":") != -1 {
		s := strings.SplitN(resource, ":", 2)
		result.ResourceType = s[0]
		result.ResourceId = s[1]
	}
	return result, nil
}

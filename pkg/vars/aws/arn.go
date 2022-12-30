package aws

import (
	"fmt"
	"strings"
)

type Arn struct {
	Arn          string
	Partition    string
	Service      string
	Region       string
	Account      string
	Resource     string
	ResourceType string
}

func ParseArn(arn string) (Arn, error) {
	// http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html
	if strings.Contains(arn, "*") {
		return Arn{}, fmt.Errorf("%s is not a valid ARN. Use of wildcard is not allowed", arn)
	}
	elements := strings.SplitN(arn, ":", 6)
	if len(elements) < 6 {
		return Arn{}, fmt.Errorf("%s is not a valid arn", arn)
	}
	var result Arn
	result.Arn = elements[0]
	result.Partition = elements[1]
	result.Service = elements[2]
	result.Region = elements[3]
	result.Account = elements[4]
	result.Resource = elements[5]

	if strings.Index(result.Resource, "/") != -1 {
		s := strings.SplitN(result.Resource, "/", 2)
		result.ResourceType = s[0]
		result.Resource = s[1]
	} else if strings.Index(result.Resource, ":") != -1 {
		s := strings.SplitN(result.Resource, ":", 2)
		result.ResourceType = s[0]
		result.Resource = s[1]
	}
	return result, nil
}

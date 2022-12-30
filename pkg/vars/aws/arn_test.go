package aws

import (
	"reflect"
	"testing"
)

func TestParseArn(t *testing.T) {
	arns := map[string]Arn{
		//Full ARN
		"arn:aws:iam:eu-central-1:123456789012:user/Development/product_1234": {
			Arn:          "arn",
			Partition:    "aws",
			Service:      "iam",
			Region:       "eu-central-1",
			Account:      "123456789012",
			Resource:     "Development/product_1234",
			ResourceType: "user",
		},
		//ARN without region
		"arn:aws:iam::123456789012:user/Development/product_1234": {
			Arn:          "arn",
			Partition:    "aws",
			Service:      "iam",
			Region:       "",
			Account:      "123456789012",
			Resource:     "Development/product_1234",
			ResourceType: "user",
		},
		//ARN without resource type
		"arn:aws:iam:eu-central-1:123456789012:i-1234567890abcdef0": {
			Arn:          "arn",
			Partition:    "aws",
			Service:      "iam",
			Region:       "eu-central-1",
			Account:      "123456789012",
			Resource:     "i-1234567890abcdef0",
			ResourceType: "",
		},
	}
	for strArn, expectedArn := range arns {
		resultArn, err := ParseArn(strArn)
		if err != nil {
			t.Errorf("Can't parse ARN: %s", strArn)
		}
		if !reflect.DeepEqual(resultArn, expectedArn) {
			t.Errorf("ARNs should match but %s != %s", resultArn, expectedArn)
		}
	}

	//Check error when using wildcards
	incorrectArn := "arn:aws:iam:eu-central-1:123456789012:user/Development/product_1234/*"
	_, err := ParseArn(incorrectArn)
	if err == nil {
		t.Errorf("ARN uses wildcard and should throw an error: %s", incorrectArn)
	}
}

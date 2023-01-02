package aws

import (
	"context"
	"testing"
)

func TestGetAwsSecretsManagerSecret(t *testing.T) {
	fcf := NewFakeClientFactory()
	tp := "test-profile"
	tr := "eu-west-1"
	ts := "alias/test"
	tc := "test-content"
	completeArn := "arn:aws:kms:eu-central-1:000000000000:alias/test"

	//Setup secrets database
	fcf.Secrets = map[string]string{ts: tc}

	//Check if getting an AWS secret works
	sc, err := GetAwsSecretsManagerSecret(context.TODO(), fcf, &tp, &tr, ts)
	if err != nil {
		t.Errorf("Getting an AWS secret failed")
	}

	if sc != tc {
		t.Errorf("Content of secret is unexpected. Should be %s but is %s", tc, sc)
	}

	//Check if empty profile works
	_, err = GetAwsSecretsManagerSecret(context.TODO(), fcf, nil, &tr, ts)
	if err != nil {
		t.Errorf("Getting an AWS secret with an empty profile throws an error: %s", err.Error())
	}

	//Check if empty region with non-ARN secret throws an error
	_, err = GetAwsSecretsManagerSecret(context.TODO(), fcf, &tp, nil, ts)
	if err == nil {
		t.Errorf("Getting an AWS secret with an empty region and a non-ARN secret name should an error")
	}

	//Check if empty region with ARN secret works
	_, err = GetAwsSecretsManagerSecret(context.TODO(), fcf, &tp, nil, completeArn)
	if err != nil {
		t.Errorf("Getting an AWS secret with an empty region and a ARN secret name throws an error: %s", err)
	}

	//Check if wrong secret arn throws an error
	_, err = GetAwsSecretsManagerSecret(context.TODO(), fcf, &tp, &tr, "")
	if err == nil {
		t.Errorf("Trying to get a secret with an empty name should throw an error")
	}
}

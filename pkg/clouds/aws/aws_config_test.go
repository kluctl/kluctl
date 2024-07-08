package aws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	status2 "github.com/kluctl/kluctl/lib/status"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func buildConfigPath(t *testing.T) string {
	p := filepath.Join(t.TempDir(), t.Name()+".cfg")
	err := os.MkdirAll(filepath.Dir(p), 0o777)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func addProfile(t *testing.T, configPath string, profileName string, keyId string) {
	newConfig := fmt.Sprintf(`
[profile %s]
region = eu-central-1
`, profileName)

	newCredentials := fmt.Sprintf(`
[%s]
aws_access_key_id = %s
aws_secret_access_key = %s
`, profileName, keyId, keyId)

	oldConfig, _ := os.ReadFile(configPath)
	oldCredentials, _ := os.ReadFile(configPath + ".creds")

	err := os.WriteFile(configPath, []byte(string(oldConfig)+newConfig), 0o666)
	assert.NoError(t, err)
	err = os.WriteFile(configPath+".creds", []byte(string(oldCredentials)+newCredentials), 0o666)
	assert.NoError(t, err)

	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", configPath+".creds")
}

func TestAwsConfig(t *testing.T) {
	type testCase struct {
		noK8s bool

		configProfiles  []string
		serviceAccounts []string

		envProfile string
		argProfile string

		awsConfigProfile string
		awsConfigSA      string

		expectedProvider    aws.CredentialsProvider
		expectedKey         string
		expectedStatusLines []string
	}

	testCases := []testCase{
		{configProfiles: []string{}, envProfile: "", noK8s: true, expectedProvider: &ec2rolecreds.Provider{}},
		{configProfiles: []string{}, envProfile: "", expectedProvider: &ec2rolecreds.Provider{}},
		{configProfiles: []string{"p1"}, envProfile: "", expectedProvider: &ec2rolecreds.Provider{}},
		{configProfiles: []string{"p1"}, envProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p1"},
		{configProfiles: []string{"p1", "p2"}, envProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p1"},
		{configProfiles: []string{"p1", "p2"}, envProfile: "p2", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p2"},
		{configProfiles: []string{"p1", "p2"}, awsConfigProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p1"},
		{configProfiles: []string{"p1", "p2"}, awsConfigProfile: "p2", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p2"},
		{configProfiles: []string{"p1", "p2"}, envProfile: "p1", awsConfigProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p1"},
		{configProfiles: []string{"p1", "p2"}, envProfile: "p1", awsConfigProfile: "p2", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p1"},
		{configProfiles: []string{"p1", "p2"}, envProfile: "p2", awsConfigProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p2"},
		{configProfiles: []string{"p1", "p2"}, envProfile: "p3", awsConfigProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p1"},
		{configProfiles: []string{"p1", "p2"}, envProfile: "p2", awsConfigProfile: "p3", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p2"},
		{configProfiles: []string{"p1", "p2"}, envProfile: "p2", awsConfigProfile: "p3", argProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p2"},
		{configProfiles: []string{"p1", "p2"}, awsConfigProfile: "p3", argProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p1"},
		{configProfiles: []string{"p1", "p2"}, awsConfigProfile: "p3", expectedProvider: &ec2rolecreds.Provider{}},

		{configProfiles: []string{"p1", "p2"}, awsConfigSA: "sa", expectedProvider: &ec2rolecreds.Provider{}, expectedStatusLines: []string{"IRSA credentials could not be generated from the service account default/sa: serviceaccounts \"sa\" not found"}},
		{configProfiles: []string{"p1", "p2"}, awsConfigSA: "sa", envProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p1"},
		{configProfiles: []string{"p1", "p2"}, awsConfigSA: "sa", awsConfigProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p1"},
		{configProfiles: []string{"p1", "p2"}, serviceAccounts: []string{"sa"}, awsConfigSA: "sa", awsConfigProfile: "p1", expectedProvider: &credentials.StaticCredentialsProvider{}, expectedKey: "p1"},
		{configProfiles: []string{"p1", "p2"}, serviceAccounts: []string{"sa"}, awsConfigSA: "sa", expectedProvider: &stscreds.WebIdentityRoleProvider{}},
		{configProfiles: []string{"p1", "p2"}, serviceAccounts: []string{"sa"}, awsConfigSA: "sa", awsConfigProfile: "p3", expectedProvider: &stscreds.WebIdentityRoleProvider{}},
	}

	k := test_utils.CreateEnvTestCluster("default")
	err := k.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		k.Stop()
	})

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			configPath := buildConfigPath(t)
			for _, p := range tc.configProfiles {
				addProfile(t, configPath, p, p)
			}
			if tc.envProfile != "" {
				t.Setenv("AWS_PROFILE", tc.envProfile)
			}

			c := k.Client
			if tc.noK8s {
				c = nil
			}

			for _, saName := range tc.serviceAccounts {
				sa := corev1.ServiceAccount{
					ObjectMeta: v1.ObjectMeta{
						Name:      saName,
						Namespace: "default",
						Annotations: map[string]string{
							"eks.amazonaws.com/role-arn": "arn:aws:iam::123456:role/test-role",
						},
					},
				}
				err := k.Client.Create(context.Background(), &sa)
				assert.NoError(t, err)
				t.Cleanup(func() {
					_ = k.Client.Delete(context.Background(), &sa)
				})
			}

			var argProfile *string
			if tc.argProfile != "" {
				argProfile = &tc.argProfile
			}

			var awsConfig types.AwsConfig
			if tc.awsConfigProfile != "" {
				awsConfig.Profile = &tc.awsConfigProfile
			}
			if tc.awsConfigSA != "" {
				awsConfig.ServiceAccount = &types.ServiceAccountRef{
					Name:      tc.awsConfigSA,
					Namespace: "default",
				}
			}

			var sls []string
			sh := status2.NewSimpleStatusHandler(func(level status2.Level, message string) {
				sls = append(sls, message)
			}, false)
			ctx := status2.NewContext(context.Background(), sh)

			cfg, err := LoadAwsConfigHelper(ctx, c, &awsConfig, argProfile)
			assert.NoError(t, err)
			assert.IsType(t, &aws.CredentialsCache{}, cfg.Credentials)

			cp := cfg.Credentials.(*aws.CredentialsCache)
			if !cp.IsCredentialsProvider(tc.expectedProvider) {
				assert.Fail(t, fmt.Sprintf("provider is not the expected one: %s", reflect.TypeOf(tc.expectedProvider).Name()))
			}

			if tc.expectedKey != "" {
				creds, err := cp.Retrieve(context.Background())
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedKey, creds.AccessKeyID)
			}
			if tc.expectedStatusLines != nil {
				assert.Equal(t, tc.expectedStatusLines, sls)
			}
		})
	}
}

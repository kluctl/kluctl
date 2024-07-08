package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/kluctl/kluctl/v2/lib/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func isConfiguredViaEnv(cfg config.EnvConfig) bool {
	if cfg.SharedConfigProfile != "" {
		return true
	}
	if cfg.Credentials.HasKeys() {
		return true
	}
	if cfg.WebIdentityTokenFilePath != "" {
		return true
	}
	return false
}

// LoadAwsConfigHelper will try to load the profile given either by profile or by awsConfig.Profile and only if this succeeds (the profile exists),
// it will use it to perform default config loading. The reason for this is that non-existent profiles is expected in Kluctl,
// at it might run on an environment that does not have the profile configured, in which case it should not error out later (due to it using a non-existing profile).
// This helper will also try to load service account based web identity configuration.
func LoadAwsConfigHelper(ctx context.Context, c client.Client, awsConfig *types.AwsConfig, profile *string, optFnsIn ...func(*config.LoadOptions) error) (aws.Config, error) {
	envCfg, err := config.NewEnvConfig()
	if err != nil {
		return aws.Config{}, err
	}
	if isConfiguredViaEnv(envCfg) {
		cfg, err := config.LoadDefaultConfig(context.Background(), optFnsIn...)
		if err == nil {
			return cfg, err
		}
		if _, ok := err.(config.SharedConfigProfileNotExistError); !ok {
			return aws.Config{}, nil
		}
	}

	var configOpts []func(*config.LoadOptions) error

	if profile == nil && awsConfig != nil {
		profile = awsConfig.Profile
	}

	profileOk := false
	if profile != nil {
		_, err := config.LoadSharedConfigProfile(ctx, *profile, func(options *config.LoadSharedConfigOptions) {
			if envCfg.SharedConfigFile != "" {
				options.ConfigFiles = append(options.ConfigFiles, envCfg.SharedConfigFile)
			}
			if envCfg.SharedCredentialsFile != "" {
				options.CredentialsFiles = append(options.CredentialsFiles, envCfg.SharedCredentialsFile)
			}
		})
		if err != nil {
			status.WarningOncef(ctx, "aws-profile-"+*profile, "The AWS profile %s could not be loaded, reverting to default handling: %s", *profile, err)
		} else {
			profileOk = true
			configOpts = append(configOpts, config.WithSharedConfigProfile(*profile))
		}
	}
	if !profileOk && c != nil && awsConfig != nil && awsConfig.ServiceAccount != nil {
		creds, err := BuildCredentialsFromServiceAccount(ctx, c, awsConfig.ServiceAccount.Name, awsConfig.ServiceAccount.Namespace, "kluctl")
		if err != nil {
			status.WarningOncef(ctx, "aws-service-account-"+awsConfig.ServiceAccount.Namespace+"-"+awsConfig.ServiceAccount.Name,
				"IRSA credentials could not be generated from the service account %s/%s: %s", awsConfig.ServiceAccount.Namespace, awsConfig.ServiceAccount.Name, err)
		} else {
			configOpts = append(configOpts, config.WithCredentialsProvider(creds))
		}
	}

	configOpts = append(configOpts, optFnsIn...)

	cfg, err := config.LoadDefaultConfig(context.Background(), configOpts...)
	return cfg, err
}

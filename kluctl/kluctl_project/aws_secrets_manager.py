import base64
import os

import boto3
from botocore.exceptions import ClientError, BotoCoreError

from kluctl.utils.aws_utils import parse_arn
from kluctl.utils.exceptions import InvalidKluctlProjectConfig


def get_aws_secrets_manager_secret(profile, region_name, secret_name):
    if "AWS_PROFILE" in os.environ:
        # Environment variable always takes precedence
        profile = None
    session = boto3.session.Session(profile_name=profile)

    if region_name is None:
        try:
            arn = parse_arn(secret_name)
            region_name = arn["region"]
        except:
            raise InvalidKluctlProjectConfig("When omitting the AWS region, the secret name must be a valid ARN")

    sm_client = session.client(
        service_name='secretsmanager',
        region_name=region_name
    )
    try:
        secret_value = sm_client.get_secret_value(SecretId=secret_name)
    except BotoCoreError as e:
        raise InvalidKluctlProjectConfig("Getting secret %s from AWS secrets manager failed: %s" % (secret_name, str(e)))
    except ClientError as e:
        raise InvalidKluctlProjectConfig("Getting secret %s from AWS secrets manager failed: %s" % (secret_name, str(e)))

    if 'SecretString' in secret_value:
        secret = secret_value['SecretString']
    else:
        secret = base64.b64decode(secret_value['SecretBinary']).decode("utf-8")

    return secret

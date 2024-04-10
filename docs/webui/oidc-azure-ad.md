<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: Azure AD Integration
linkTitle: Azure AD Integration
description: Azure AD Integration
weight: 30
---
-->

# Azure AD Integration

Azure AD can be integrated via the [OIDC integration](./installation.md#oidc-integration).

## Configure a new Azure AD App registration
### Add a new Azure AD App registration

1. From the `Azure Active Directory` > `App registrations` menu, choose `+ New registration`
2. Enter a `Name` for the application (e.g. `Kluctl Webui`).
3. Specify who can use the application (e.g. `Accounts in this organizational directory only`).
4. Enter Redirect URI (optional) as follows (replacing `my-kluctl-webui-url` with your Kluctl Webui URL), then choose `Add`.
    - **Platform:** `Web`
    - **Redirect URI:** https://`<my-kluctl-webui-url>`/auth/callback
5. When registration finishes, the Azure portal displays the app registration's Overview pane. You see the Application (client) ID.
   ![Azure App registration's Overview](https://kluctl.io/images/webui/azure-app-registration-overview.png "Azure App registration's Overview")

### Add credentials a new Azure AD App registration

1. From the `Certificates & secrets` menu, choose `+ New client secret`
2. Enter a `Name` for the secret (e.g. `Kluctl Webui SSO`).
    - Make sure to copy and save generated value. This is a value for the `oidc-client-secret`.
      ![Azure App registration's Secret](https://kluctl.io/images/webui/azure-app-registration-secret.png "Azure App registration's Secret")

### Setup permissions for Azure AD Application

1. From the `API permissions` menu, choose `+ Add a permission`
2. Find `User.Read` permission (under `Microsoft Graph`) and grant it to the created application:
   ![Azure AD API permissions](https://kluctl.io/images/webui/azure-api-permissions.png "Azure AD API permissions")
3. From the `Token Configuration` menu, choose `+ Add groups claim`
   ![Azure AD token configuration](https://kluctl.io/images/webui/azure-token-configuration.png "Azure AD token configuration")

## Associate an Azure AD group to your Azure AD App registration

1. From the `Azure Active Directory` > `Enterprise applications` menu, search the App that you created (e.g. `Kluctl Webui`).
    - An Enterprise application with the same name of the Azure AD App registration is created when you add a new Azure AD App registration.
2. From the `Users and groups` menu of the app, add any users or groups requiring access to the service.
   ![Azure Enterprise SAML Users](https://kluctl.io/images/webui/azure-enterprise-users.png "Azure Enterprise SAML Users")

## Configure the Kluctl Webui to use the new Azure AD App registration

Use the following configuration when [installing](./installation.md) the Webui. Replace occurrences of 
`<directory_tenant_id>`, `<client_id>`, `<my-kluctl-webui-url>` and `<admin_group_id>` with the appropriate values from
above.

```yaml
deployments:
  - path: secrets 
  - git:
      url: https://github.com/kluctl/kluctl.git
      subDir: install/webui
      ref:
         tag: v2.24.1
    vars:
      - values:
          args:
            webui_args:
              - --auth-oidc-issuer-url=https://login.microsoftonline.com/<directory_tenant_id>/v2.0
              - --auth-oidc-client-id=<client_id>
              - --auth-oidc-scope=openid
              - --auth-oidc-scope=profile
              - --auth-oidc-scope=email
              - --auth-oidc-redirect-url=https://<my-kluctl-webui-url>/auth/callback
              - --auth-oidc-group-claim=groups
              - --auth-oidc-admins-group=<admin_group_id>
```

Also, add `webui-secrets.yaml` inside the `secrets` subdirectory:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: webui-secret
  namespace: kluctl-system
stringData:
  oidc-client-secret: "<client_secret>"
```

Please note that the client secret is sensitive data and should not be added unencrypted to you git repository.
Consider encrypting it via [SOPS](../kluctl/deployments/sops.md).

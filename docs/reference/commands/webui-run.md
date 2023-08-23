<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "webui run"
linkTitle: "webui run"
weight: 10
description: >
    webui command
---
-->

## Command
<!-- BEGIN SECTION "webui run" "Usage" false -->
Usage: kluctl webui run [flags]

Run the Kluctl Webui
<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "webui run" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --all-contexts                Use all Kubernetes contexts found in the kubeconfig.
      --context stringArray         List of kubernetes contexts to use.
      --host string                 Host to bind to. Pass an empty string to bind to all addresses. (default
                                    "localhost")
      --in-cluster                  This enables in-cluster functionality. This also enforces authentication.
      --in-cluster-context string   The context to use fo in-cluster functionality.
      --only-api                    Only serve API without the actual UI.
      --port int                    Port to bind to. (default 8080)

```
<!-- END SECTION -->

<!-- BEGIN SECTION "webui run" "Auth arguments" true -->
```
Auth arguments:
  Configure authentication.

      --auth-admin-rbac-user string            Specify the RBAC user to use for admin access. (default
                                               "kluctl-webui-admin")
      --auth-logout-return-param string        Specify the parameter name to pass to the logout redirect url,
                                               containing the return URL to redirect back.
      --auth-logout-url string                 Specify the logout URL, to which the user should be redirected
                                               after clearing the Kluctl Webui session.
      --auth-oidc-admins-group stringArray     Specify admins group names.'
      --auth-oidc-client-id string             Specify the ClientID.
      --auth-oidc-client-secret-key string     Specify the secret name for the ClientSecret. (default
                                               "oidc-client-secret")
      --auth-oidc-client-secret-name string    Specify the secret name for the ClientSecret. (default "webui-secret")
      --auth-oidc-display-name string          Specify the name of the OIDC provider to be displayed on the login
                                               page. (default "OpenID Connect")
      --auth-oidc-group-claim string           Specify claim for the groups.' (default "groups")
      --auth-oidc-issuer-url string            Specify the OIDC provider's issuer URL.
      --auth-oidc-param stringArray            Specify additional parameters to be passed to the authorize endpoint.
      --auth-oidc-redirect-url string          Specify the redirect URL.
      --auth-oidc-scope stringArray            Specify the scopes.
      --auth-oidc-user-claim string            Specify claim for the username.' (default "email")
      --auth-oidc-viewers-group stringArray    Specify viewers group names.'
      --auth-secret-key string                 Specify the secret key for the secret used for internal encryption
                                               of tokens and cookies. (default "auth-secret")
      --auth-secret-name string                Specify the secret name for the secret used for internal encryption
                                               of tokens and cookies. (default "webui-secret")
      --auth-static-admin-secret-key string    Specify the secret key for the admin password. (default
                                               "admin-password")
      --auth-static-login-enabled              Enable the admin user. (default true)
      --auth-static-login-secret-name string   Specify the secret name for the admin and viewer passwords.
                                               (default "webui-secret")
      --auth-static-viewer-secret-key string   Specify the secret key for the viewer password. (default
                                               "viewer-password")
      --auth-viewer-rbac-user string           Specify the RBAC user to use for viewer access. (default
                                               "kluctl-webui-viewer")

```
<!-- END SECTION -->

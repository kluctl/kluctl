# Kluctl Web UI

React, TypeScript, Vite and shadcn (Radix). The home view groups deployments by repository and project directory. GitOps deployments come first; CLI targets and historical targets are labeled separately. Status filters count targets, not individual messages. The run history uses TanStack Table.

## Development

Use Node.js 24 or later and the pnpm version pinned in `package.json`.

```sh
pnpm install --frozen-lockfile
pnpm dev
pnpm lint
pnpm test
```

Vite proxies `/auth`, `/api` and their WebSockets to `http://localhost:9090`. Run `kluctl webui run --only-api --port 9090` in another terminal with your preferred Kubernetes context. Add UI primitives with `pnpm dlx shadcn@latest add <component>` from this directory.

## Try it on kind

From the repo root, use a dedicated kubeconfig so the demo never uses your current cluster. Build `bin/kluctl` as described below, then run:

```sh
DEMO_DIR=$(mktemp -d)
kind create cluster --name kluctl-ui --kubeconfig "$DEMO_DIR/kubeconfig"
bin/kluctl controller install --yes --kluctl-version v2.28.2 --kubeconfig "$DEMO_DIR/kubeconfig"
kubectl --kubeconfig "$DEMO_DIR/kubeconfig" -n kluctl-system rollout status deployment/kluctl-controller
bash pkg/webui/ui/demo/seed.sh "$DEMO_DIR/kubeconfig"
bin/kluctl webui run --kubeconfig "$DEMO_DIR/kubeconfig"
```

Open [http://localhost:8080](http://localhost:8080). The seed script creates six `KluctlDeployment`s sourced from [kluctl-examples](https://github.com/kluctl/kluctl-examples). After reconciliation, you'll see a healthy target, a prepare failure, Kubernetes Pod Security warnings, a replica drift (`2` instead of `3`), a deployment awaiting approval, and a suspended deployment. Drift and validation can take another 45-second reconciliation cycle to settle. To use Vite instead, run the last command with `--only-api --port 9090` and open the URL from `pnpm dev`.

Clean up the disposable cluster with `kind delete cluster --name kluctl-ui --kubeconfig "$DEMO_DIR/kubeconfig"`.

## Embedded build and snapshots

From the repo root, run `make build-webui` before `make build-bin`. The static Vite bundle is written to `pkg/webui/ui/build` and embedded in the Go binary. `kluctl webui run` serves it; `kluctl webui build --path <directory>` exports it with result data. Serve the export over HTTP (for example `python3 -m http.server --directory <directory>`). A snapshot can contain sensitive values and should not be published as a viewer-only version of the live UI. By default, the exporter stores one result per project and target; use `--max-results` to include more history.

The SPA uses hash routes and relative asset/API URLs. Existing Kluctl target and result links, including `?full=1` for the resource tree, remain valid.

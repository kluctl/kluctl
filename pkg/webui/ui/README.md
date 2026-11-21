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

## Embedded build and snapshots

From the repo root, run `make build-webui` before `make build-bin`. The static Vite bundle is written to `pkg/webui/ui/build` and embedded in the Go binary. `kluctl webui run` serves it; `kluctl webui build --path <directory>` exports it with result data. Serve the export over HTTP (for example `python3 -m http.server --directory <directory>`). A snapshot can contain sensitive values and should not be published as a viewer-only version of the live UI. By default, the exporter stores one result per project and target; use `--max-results` to include more history.

The SPA uses hash routes and relative asset/API URLs. Existing Kluctl target and result links, including `?full=1` for the resource tree, remain valid.

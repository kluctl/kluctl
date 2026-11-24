import type { DeploymentEventData, Event as InventoryEvent } from "./domain"
import { refKey } from "./domain"
import type {
  AuthInfo,
  CommandResult,
  CommandResultSummary,
  ObjectRef,
  ShortName,
  ValidateResult,
} from "./models"

export type User = { username: string; isAdmin: boolean }
export type Capabilities = AuthInfo & { logsEnabled?: boolean }
export type ObjectVersion = "rendered" | "remote" | "applied"

declare const isStaticBuild: boolean | undefined
declare const staticResults: Map<
  string,
  CommandResult | { error: string } | null
>
declare const staticSummaries: CommandResultSummary[] | null
declare const staticKluctlDeployments: DeploymentEventData[] | null
declare const staticShortNames: ShortName[] | null

export const snapshot = typeof isStaticBuild !== "undefined" && isStaticBuild
export const base = new URL("./", window.location.href)
export const endpoint = (path: string) => new URL(path, base).href
export function socketUrl(path: string, query?: Record<string, string>) {
  const url = new URL(path, base)
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:"
  url.search = new URLSearchParams(query).toString()
  return url.href
}

export class HttpError extends Error {
  readonly status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request(
  path: string,
  init?: RequestInit,
  invalidateSession = true
) {
  const response = await fetch(endpoint(path), init)
  if (!response.ok) {
    if (response.status === 401 && invalidateSession)
      window.dispatchEvent(new Event("session-expired"))
    throw new HttpError(
      response.status,
      response.status === 404
        ? "This entry is no longer available."
        : response.status === 403
          ? "You don't have access to this entry."
          : `Request failed (${response.status}${response.statusText ? ` ${response.statusText}` : ""}).`
    )
  }
  return response
}
export async function get<T>(
  path: string,
  query?: Record<string, string>,
  signal?: AbortSignal,
  invalidateSession = true
): Promise<T> {
  const url = query ? `${path}?${new URLSearchParams(query)}` : path
  return (await request(url, { signal }, invalidateSession)).json()
}
export async function post(
  path: string,
  data: Record<string, unknown>,
  invalidateSession = true
) {
  await request(
    path,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    },
    invalidateSession
  )
}

const scripts = new Map<string, Promise<void>>()
function loadScript(path: string): Promise<void> {
  const pending = scripts.get(path)
  if (pending) return pending
  const script = document.createElement("script")
  script.src = endpoint(path)
  const promise = new Promise<void>((resolve, reject) => {
    script.onload = () => resolve()
    script.onerror = () => {
      scripts.delete(path)
      script.remove()
      reject(new Error(`Could not load snapshot data: ${path}`))
    }
    document.head.append(script)
  })
  scripts.set(path, promise)
  return promise
}

export async function snapshotEvents(): Promise<InventoryEvent[]> {
  await Promise.all([
    loadScript("staticdata/summaries.js"),
    loadScript("staticdata/kluctldeployments.js"),
  ])
  return [
    ...(staticSummaries ?? []).map((summary) => ({
      type: "update_command_result_summary" as const,
      summary,
    })),
    ...(staticKluctlDeployments ?? []).map((data) => ({
      type: "update_kluctl_deployment" as const,
      ...data,
    })),
  ]
}
export async function getShortNames(signal: AbortSignal): Promise<ShortName[]> {
  if (!snapshot)
    return (
      (await get<ShortName[] | null>("api/getShortNames", undefined, signal)) ??
      []
    )
  await loadScript("staticdata/shortnames.js")
  return staticShortNames ?? []
}
export async function getResult(
  id: string,
  signal: AbortSignal
): Promise<CommandResult> {
  if (!snapshot) return get("api/getCommandResult", { resultId: id }, signal)
  await loadScript(`staticdata/result-${encodeURIComponent(id)}.js`)
  const result = staticResults.get(id)
  if (!result) throw new Error("This run was not included in the snapshot.")
  if ("error" in result) throw new Error(result.error)
  return result
}
export function getValidation(id: string, signal: AbortSignal) {
  return get<ValidateResult>("api/getValidateResult", { resultId: id }, signal)
}
export async function getObject(
  result: CommandResult,
  ref: ObjectRef,
  version: ObjectVersion,
  signal: AbortSignal
): Promise<unknown> {
  if (snapshot)
    return result.objects?.find((o) => refKey(o.ref) === refKey(ref))?.[version]
  const { group, kind, name, namespace, version: apiVersion } = ref
  return get(
    "api/getCommandResultObject",
    {
      resultId: result.id,
      objectType: version,
      kind,
      name,
      group: group ?? "",
      version: apiVersion ?? "",
      namespace: namespace ?? "",
    },
    signal
  )
}

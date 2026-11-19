import {
  approvalOf,
  reconciliation,
  targetSignals,
  type TargetRow,
} from "./domain"

export function deploymentName(row: TargetRow) {
  return (
    row.instance?.name ||
    row.target.targetName ||
    row.target.discriminator ||
    "Default target"
  )
}

export function projectIdentity(row: TargetRow) {
  const repo = row.project.repoKey || ""
  const path = row.project.subDir === "." ? "" : row.project.subDir || ""
  const repository = repo.replace(/^(git|oci):\/\//, "")
  const unresolved =
    !!row.deployment && !repo && !path && !row.deployment.status?.projectKey
  const name =
    path.split("/").at(-1) ||
    repository.split("/").at(-1) ||
    (unresolved ? "Unresolved source" : "Local project")
  return {
    id: JSON.stringify([repo, path, unresolved]),
    name,
    repo,
    path,
    location: [repository, path].filter(Boolean).join(" / "),
  }
}

type ProjectGroup = ReturnType<typeof projectIdentity> & {
  deployments: TargetRow[]
  recorded: TargetRow[]
}

export function projectGroups(rows: TargetRow[]) {
  const groups = new Map<string, ProjectGroup>()
  for (const row of rows) {
    const project = projectIdentity(row)
    let group = groups.get(project.id)
    if (!group) {
      group = { ...project, deployments: [], recorded: [] }
      groups.set(group.id, group)
    }
    if (row.deployment) group.deployments.push(row)
    else group.recorded.push(row)
  }
  for (const group of groups.values())
    group.deployments.sort((a, b) =>
      deploymentName(a).localeCompare(deploymentName(b))
    )
  return Array.from(groups.values()).sort(
    (a, b) =>
      Number(b.deployments.length > 0) - Number(a.deployments.length > 0) ||
      a.repo.localeCompare(b.repo) ||
      a.path.localeCompare(b.path) ||
      a.name.localeCompare(b.name)
  )
}

export function matchesSearch(row: TargetRow, query: string) {
  const fields = [
    row.project.repoKey,
    row.project.subDir,
    deploymentName(row),
    row.target.targetName,
    row.target.discriminator,
    row.target.clusterId,
    row.instance?.clusterId,
    row.instance?.namespace,
    row.deployment ? "GitOps" : row.instance ? "Historical" : "CLI",
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase()
  return query
    .toLowerCase()
    .trim()
    .split(/\s+/)
    .every((word) => fields.includes(word))
}

export const filters = [
  "all",
  "errors",
  "warnings",
  "drift",
  "important",
  "issues",
] as const
export type InventoryFilter = (typeof filters)[number]
export function inventoryFilter(value: string | null): InventoryFilter {
  return filters.find((filter) => filter === value) ?? "all"
}

export function targetHealth(row: TargetRow) {
  return { ...targetSignals(row), approval: !!approvalOf(row)?.required }
}

export function matchesFilter(
  health: ReturnType<typeof targetHealth>,
  filter: InventoryFilter
) {
  switch (filter) {
    case "errors":
    case "warnings":
    case "drift":
      return health[filter] > 0
    case "important":
      return !!(
        health.errors ||
        health.warnings ||
        health.drift ||
        health.approval
      )
    case "issues":
      return !!(health.errors || health.warnings)
    default:
      return true
  }
}

export type Indicator =
  | "error"
  | "warning"
  | "drift"
  | "approval"
  | "ready"
  | "suspended"
  | "running"
  | "queued"
  | "pending"
  | "dryRun"
  | "recorded"

export function targetIndicators(row: TargetRow): Indicator[] {
  const health = targetHealth(row)
  const items: Indicator[] = []
  if (health.errors) items.push("error")
  if (health.warnings) items.push("warning")
  if (health.drift) items.push("drift")
  if (health.approval) items.push("approval")
  if (!row.deployment) return items.length ? items : ["recorded"]

  switch (reconciliation(row).label) {
    case "Suspended":
      items.unshift("suspended")
      break
    case "Reconciling":
      items.unshift("running")
      break
    case "Requested":
      items.unshift("queued")
      break
    case "Ready":
      if (!items.length)
        items.push(row.deployment.spec.dryRun ? "dryRun" : "ready")
      break
    default:
      if (!items.length) items.push("pending")
  }
  return items
}

export const quantity = (
  count: number,
  singular: string,
  plural = `${singular}s`
) => `${count} ${count === 1 ? singular : plural}`

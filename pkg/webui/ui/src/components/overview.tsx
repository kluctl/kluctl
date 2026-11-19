import { useEffect, useId, useMemo, useRef, useState } from "react"
import {
  ArrowRight,
  Box,
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  FolderGit2,
  GitCompareArrows,
  ListFilter,
  Search,
  TriangleAlert,
  X,
} from "lucide-react"
import { Link, useSearchParams } from "react-router-dom"
import { targetPath, type TargetRow } from "@/domain"
import { useSearchDraft } from "@/hooks"
import {
  deploymentName,
  inventoryFilter,
  matchesFilter,
  matchesSearch,
  projectGroups,
  quantity,
  targetHealth,
  type InventoryFilter,
} from "@/inventory-view"
import { formatDate } from "./detail-primitives"
import { TargetIndicators } from "./target-indicators"
import { Button } from "./ui/button"
import { Input } from "./ui/input"

const metrics = [
  { id: "all", label: "All targets", icon: Box, tone: "good" },
  { id: "errors", label: "With errors", icon: CircleAlert, tone: "bad" },
  { id: "warnings", label: "With warnings", icon: TriangleAlert, tone: "warn" },
  { id: "drift", label: "With drift", icon: GitCompareArrows, tone: "drift" },
] as const
const pageSize = 12

function ProjectsHero() {
  return (
    <div className="projects-hero">
      <div className="relative z-10 min-w-0">
        <p className="projects-eyebrow">Deployment workspace</p>
        <h1 className="mt-1 text-[2.5rem] leading-tight font-semibold tracking-[-0.045em]">
          Projects
        </h1>
        <p className="mt-1 text-sm text-[#c5dcd2] sm:text-[0.95rem]">
          See deployments and what needs attention.
        </p>
      </div>
      <svg
        aria-hidden="true"
        className="projects-map"
        viewBox="0 0 540 172"
        fill="none"
      >
        <defs>
          <pattern
            id="topology-dots"
            width="16"
            height="16"
            patternUnits="userSpaceOnUse"
          >
            <circle cx="1" cy="1" r="1" fill="#b5f7d3" opacity=".32" />
          </pattern>
        </defs>
        <rect width="540" height="172" fill="url(#topology-dots)" />
        <path
          d="M2 88h150m76 0h40q12 0 12-12V49q0-12 12-12h70m-94 51h116m-116 0q12 0 12 12v27q0 12 12 12h70m76-102h104m-82 51h82m-104 51h104"
          stroke="#81c4ad"
          strokeOpacity=".55"
          strokeWidth="1.25"
        />
        <rect
          x="144"
          y="62"
          width="92"
          height="52"
          rx="12"
          fill="#69c7a3"
          fillOpacity=".09"
          stroke="#7ccdb0"
          strokeOpacity=".22"
        />
        <rect
          x="150"
          y="68"
          width="80"
          height="40"
          rx="8"
          fill="#1e5143"
          stroke="#8bd2b8"
          strokeOpacity=".8"
        />
        <path
          d="m171 83 7-4 7 4v9l-7 4-7-4v-9Z m0 0 7 4 7-4m-7 4v9"
          stroke="#bdf6dc"
          strokeWidth="1.2"
          strokeLinejoin="round"
        />
        <path
          d="M195 85h18m-18 7h12"
          stroke="#bdf6dc"
          strokeOpacity=".7"
          strokeWidth="2"
          strokeLinecap="round"
        />
        <g fill="#1b4236" stroke="#71a996">
          <rect x="357" y="22" width="84" height="30" rx="7" />
          <rect x="381" y="73" width="84" height="30" rx="7" />
          <rect x="357" y="124" width="84" height="30" rx="7" />
        </g>
        <g
          stroke="#9bceb9"
          strokeOpacity=".65"
          strokeWidth="2"
          strokeLinecap="round"
        >
          <path d="M384 37h38m-14 51h38m-62 51h38" />
        </g>
        <circle cx="372" cy="37" r="3.5" fill="#9ee7c0" />
        <circle cx="396" cy="88" r="3.5" fill="#f5d192" />
        <circle cx="372" cy="139" r="3.5" fill="#c2b7f4" />
      </svg>
    </div>
  )
}

const relativeTime = new Intl.RelativeTimeFormat(undefined, {
  numeric: "auto",
  style: "short",
})
function RunTime({ value, now }: { value?: string; now: number }) {
  if (
    !value ||
    value.startsWith("0001-") ||
    !Number.isFinite(Date.parse(value))
  ) {
    return <span>No run yet</span>
  }
  const minutes = Math.max(0, Math.floor((now - Date.parse(value)) / 60_000))
  const text =
    minutes === 0
      ? "Just now"
      : minutes < 60
        ? relativeTime.format(-minutes, "minute")
        : minutes < 1440
          ? relativeTime.format(-Math.floor(minutes / 60), "hour")
          : relativeTime.format(-Math.floor(minutes / 1440), "day")
  return (
    <time dateTime={value} title={formatDate(value)}>
      {text}
    </time>
  )
}

function DeploymentRow({
  row,
  now,
  showCluster,
}: {
  row: TargetRow
  now: number
  showCluster: boolean
}) {
  const name = deploymentName(row)
  const cluster = row.instance?.clusterId || row.target.clusterId
  const lastRun = row.runs[0] ?? row.deployment?.status?.lastDeployResult
  const mode = row.deployment
    ? row.instance?.namespace
    : row.instance
      ? "Historical"
      : "CLI"
  const target =
    row.target.targetName ||
    (row.deployment && !row.target.clusterId ? "Not resolved" : "Default")
  return (
    <li className="min-w-0">
      <Link
        className="deployment-grid deployment-row group"
        to={targetPath(row)}
      >
        <span className="deployment-label min-w-0">
          <span className="deployment-icon">
            <Box aria-hidden="true" />
          </span>
          <span className="min-w-0">
            <span className="block truncate text-sm font-semibold" title={name}>
              {name}
            </span>
            <span
              className="mt-0.5 block truncate text-xs text-muted-foreground"
              title={showCluster ? `${mode} · ${cluster}` : mode}
            >
              {mode}
              {showCluster && cluster && <> · {cluster.slice(0, 8)}</>}
              {!row.deployment && <> · {quantity(row.runs.length, "run")}</>}
              {row.deployment?.spec.dryRun && <> · Dry run</>}
              {target !== name && (
                <span className="md:hidden"> · {target}</span>
              )}
            </span>
          </span>
        </span>
        <span
          className="hidden min-w-0 truncate text-sm text-muted-foreground md:block"
          title={target}
        >
          <span className="sr-only">Target: </span>
          {target}
        </span>
        <span className="deployment-indicators min-w-0">
          <TargetIndicators row={row} />
        </span>
        <span className="deployment-runtime min-w-0 text-[0.8125rem] text-muted-foreground">
          <span className="sr-only">Last run: </span>
          <RunTime value={lastRun?.commandInfo?.startTime} now={now} />
        </span>
        <ChevronRight
          aria-hidden="true"
          className="deployment-chevron size-4 text-muted-foreground transition-transform motion-safe:group-hover:translate-x-0.5"
        />
      </Link>
    </li>
  )
}

function DeploymentRows({ rows, now }: { rows: TargetRow[]; now: number }) {
  const key = (row: TargetRow) =>
    JSON.stringify([deploymentName(row), row.instance?.namespace])
  const names = new Set<string>()
  const ambiguous = new Set<string>()
  for (const row of rows) {
    const identity = key(row)
    if (names.has(identity)) ambiguous.add(identity)
    names.add(identity)
  }
  return (
    <ul
      className="deployment-list divide-y"
      aria-label={rows[0]?.deployment ? "Deployments" : "Recorded targets"}
    >
      {rows.map((row) => (
        <DeploymentRow
          row={row}
          key={row.id}
          now={now}
          showCluster={ambiguous.has(key(row))}
        />
      ))}
    </ul>
  )
}

function Project({
  project,
  now,
  filtered,
}: {
  project: ReturnType<typeof projectGroups>[number]
  now: number
  filtered: boolean
}) {
  const id = useId()
  const hasDeployments = project.deployments.length > 0
  const count = hasDeployments
    ? project.deployments.length
    : project.recorded.length
  const countLabel = quantity(
    count,
    hasDeployments ? "deployment" : "recorded target"
  )
  return (
    <article className="project-panel" aria-labelledby={id}>
      <header className="project-header">
        <span className="project-icon">
          <FolderGit2 aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <h3
            id={id}
            className="truncate text-[0.975rem] font-semibold tracking-tight"
            title={project.name}
          >
            {project.name}
          </h3>
          {project.location && (
            <p
              className="mt-0.5 truncate text-xs text-muted-foreground"
              title={project.location}
            >
              {project.location}
            </p>
          )}
        </div>
        <span className="project-count" title={countLabel}>
          <span className="sr-only sm:not-sr-only">{countLabel}</span>
          <span aria-hidden="true" className="sm:hidden">
            {count}
          </span>
        </span>
      </header>
      {hasDeployments && (
        <DeploymentRows rows={project.deployments} now={now} />
      )}
      {!!project.recorded.length &&
        (hasDeployments ? (
          <details
            key={String(filtered)}
            open={filtered || undefined}
            className="group/recorded border-t"
          >
            <summary className="recorded-toggle">
              <ChevronRight
                aria-hidden="true"
                className="size-3.5 group-open/recorded:rotate-90"
              />
              {quantity(project.recorded.length, "recorded target")}
            </summary>
            <DeploymentRows rows={project.recorded} now={now} />
          </details>
        ) : (
          <DeploymentRows rows={project.recorded} now={now} />
        ))}
    </article>
  )
}

export function Overview({
  rows,
  loaded,
}: {
  rows: TargetRow[]
  loaded: boolean
}) {
  const [params, setParams] = useSearchParams()
  const [now, setNow] = useState(Date.now)
  const search = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useSearchDraft(params.get("q") ?? "")
  const mac = /Mac|iPhone|iPad|iPod/.test(navigator.platform)
  const filter = inventoryFilter(params.get("attention"))
  const filtered = !!query.trim() || filter !== "all"
  const requestedPage = Math.max(1, Math.floor(Number(params.get("page")) || 1))
  const all = useMemo(
    () => rows.map((row) => ({ row, health: targetHealth(row) })),
    [rows]
  )
  const counts = useMemo(
    () => ({
      all: all.length,
      errors: all.filter(({ health }) => health.errors > 0).length,
      warnings: all.filter(({ health }) => health.warnings > 0).length,
      drift: all.filter(({ health }) => health.drift > 0).length,
    }),
    [all]
  )
  const matches = useMemo(
    () =>
      all
        .filter(
          ({ row, health }) =>
            matchesFilter(health, filter) && matchesSearch(row, query)
        )
        .map(({ row }) => row),
    [all, filter, query]
  )
  const projects = useMemo(() => projectGroups(matches), [matches])
  const deploymentCount = matches.filter((row) => row.deployment).length
  const recordedCount = matches.length - deploymentCount
  const pages = Math.ceil(projects.length / pageSize)
  const page = Math.max(1, Math.min(requestedPage, pages))
  const visible = projects.slice((page - 1) * pageSize, page * pageSize)

  useEffect(() => {
    const interval = window.setInterval(() => setNow(Date.now()), 30_000)
    return () => window.clearInterval(interval)
  }, [])
  useEffect(() => {
    function shortcut(event: KeyboardEvent) {
      const target = event.target
      if (
        event.key.toLowerCase() !== "k" ||
        !(event.metaKey || event.ctrlKey) ||
        event.altKey ||
        event.shiftKey ||
        event.defaultPrevented ||
        (target instanceof HTMLElement &&
          (target.isContentEditable ||
            target.closest(
              "input, textarea, select, [role=dialog], [role=menu]"
            )))
      )
        return
      if (!search.current) return
      event.preventDefault()
      search.current.focus()
    }
    window.addEventListener("keydown", shortcut)
    return () => window.removeEventListener("keydown", shortcut)
  }, [])

  function setFilter(next: InventoryFilter) {
    update("attention", next === "all" || next === filter ? "" : next)
  }
  function update(key: string, value: string) {
    if (key === "q") setQuery(value)
    setParams(
      (previous) => {
        const next = new URLSearchParams(previous)
        const text = key === "q" ? value : query
        if (text) next.set("q", text)
        else next.delete("q")
        if (key !== "q") {
          if (value) next.set(key, value)
          else next.delete(key)
        }
        if (key !== "page") next.delete("page")
        return next
      },
      { replace: true }
    )
  }

  function goToPage(page: number) {
    update("page", String(page))
    document.getElementById("inventory-heading")?.focus()
  }

  return (
    <section className="space-y-4 sm:space-y-5">
      <ProjectsHero />
      {loaded ? (
        <div
          className="inventory-metrics"
          aria-label="Filter targets by status"
          role="group"
        >
          {metrics.map(({ id, label, icon: Icon, tone }) => (
            <button
              key={id}
              type="button"
              className="inventory-metric"
              data-tone={tone}
              aria-pressed={filter === id}
              onClick={() => setFilter(id)}
            >
              <span className="metric-icon" data-tone={tone}>
                <Icon aria-hidden="true" />
              </span>
              <span className="min-w-0 text-left">
                <span className="block text-[1.375rem] leading-6 font-semibold tracking-tight tabular-nums">
                  {counts[id]}
                </span>
                <span className="block text-xs font-medium text-muted-foreground">
                  {label}
                </span>
              </span>
            </button>
          ))}
        </div>
      ) : (
        <div className="inventory-metrics" aria-hidden="true">
          {metrics.map(({ id }) => (
            <div key={id} className="inventory-metric">
              <span className="h-9 w-24 rounded-md bg-muted motion-safe:animate-pulse" />
            </div>
          ))}
        </div>
      )}
      <section aria-labelledby="inventory-heading" className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
          <div>
            <h2
              id="inventory-heading"
              tabIndex={-1}
              className="text-lg font-semibold tracking-tight outline-none"
            >
              Project inventory
            </h2>
            <p
              role="status"
              className="mt-0.5 text-[0.8125rem] text-muted-foreground"
            >
              {loaded
                ? [
                    quantity(projects.length, "project"),
                    deploymentCount || !recordedCount
                      ? quantity(deploymentCount, "deployment")
                      : "",
                    recordedCount
                      ? quantity(recordedCount, "recorded target")
                      : "",
                  ]
                    .filter(Boolean)
                    .join(" · ")
                : "Loading projects…"}
            </p>
          </div>
          {loaded && (
            <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
              <div className="project-search relative min-w-48 flex-1 sm:w-72">
                <Search
                  aria-hidden="true"
                  className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
                />
                <Input
                  ref={search}
                  className="h-10 bg-card pr-14 pl-9 dark:bg-card"
                  aria-label="Search projects or deployments"
                  aria-keyshortcuts={mac ? "Meta+K" : "Control+K"}
                  placeholder="Search…"
                  value={query}
                  onChange={(event) => update("q", event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Escape" && query) update("q", "")
                  }}
                />
                {query ? (
                  <button
                    type="button"
                    className="search-clear"
                    aria-label="Clear search"
                    onClick={() => {
                      update("q", "")
                      search.current?.focus()
                    }}
                  >
                    <X aria-hidden="true" className="size-4" />
                  </button>
                ) : (
                  <kbd aria-hidden="true" className="search-shortcut">
                    {mac ? "⌘ K" : "Ctrl K"}
                  </kbd>
                )}
              </div>
              <Button
                type="button"
                variant="outline"
                className="h-10 gap-2 px-3 aria-pressed:border-primary/35 aria-pressed:bg-accent aria-pressed:text-primary"
                aria-pressed={filter === "important"}
                onClick={() => setFilter("important")}
              >
                <ListFilter aria-hidden="true" /> Needs attention
              </Button>
            </div>
          )}
        </div>
        {loaded && projects.length > 0 ? (
          <>
            <div
              aria-hidden="true"
              className="deployment-grid inventory-columns"
            >
              <span>Deployment</span>
              <span>Target</span>
              <span>Status</span>
              <span>Last run</span>
            </div>
            <div className="space-y-3">
              {visible.map((project) => (
                <Project
                  key={project.id}
                  project={project}
                  now={now}
                  filtered={filtered}
                />
              ))}
            </div>
            {pages > 1 && (
              <nav
                className="flex flex-wrap items-center justify-between gap-2 text-sm text-muted-foreground"
                aria-label="Project pages"
              >
                <span>
                  Page {page} of {pages}
                </span>
                <div className="flex gap-1.5">
                  <Button
                    variant="outline"
                    type="button"
                    disabled={page === 1}
                    onClick={() => goToPage(page - 1)}
                  >
                    <ChevronLeft aria-hidden="true" /> Previous
                  </Button>
                  <Button
                    variant="outline"
                    type="button"
                    disabled={page === pages}
                    onClick={() => goToPage(page + 1)}
                  >
                    Next <ChevronRight aria-hidden="true" />
                  </Button>
                </div>
              </nav>
            )}
          </>
        ) : loaded ? (
          <div className="inventory-empty">
            <span className="project-icon size-11">
              {filtered ? (
                <Search aria-hidden="true" />
              ) : (
                <FolderGit2 aria-hidden="true" />
              )}
            </span>
            <h3 className="mt-3 font-semibold">
              {filtered ? "No matching targets" : "No projects yet"}
            </h3>
            <p className="max-w-sm text-sm text-muted-foreground">
              {filtered
                ? "Try a different search or clear the status filter."
                : "Projects appear when the controller sees a KluctlDeployment or a Kluctl run is saved."}
            </p>
            {filtered ? (
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setQuery("")
                  setParams({}, { replace: true })
                  search.current?.focus()
                }}
              >
                Clear filters
              </Button>
            ) : (
              <Button asChild variant="outline">
                <a href="https://kluctl.io/docs/gitops/">
                  GitOps documentation <ArrowRight aria-hidden="true" />
                </a>
              </Button>
            )}
          </div>
        ) : (
          <div className="project-panel p-5" aria-hidden="true">
            <div className="h-5 w-40 rounded bg-muted motion-safe:animate-pulse" />
            <div className="mt-6 h-8 rounded bg-muted/70 motion-safe:animate-pulse" />
            <div className="mt-3 h-8 rounded bg-muted/70 motion-safe:animate-pulse" />
          </div>
        )}
      </section>
    </section>
  )
}

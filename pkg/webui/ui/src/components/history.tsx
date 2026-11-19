import { useCallback, useMemo, useState } from "react"
import {
  getCoreRowModel,
  getPaginationRowModel,
  useReactTable,
  type ColumnDef,
  type PaginationState,
} from "@tanstack/react-table"
import { ChevronLeft, ChevronRight, List } from "lucide-react"
import { Link, useNavigate, useSearchParams } from "react-router-dom"
import { historyPath, runOutcome, type TargetRow } from "@/domain"
import { useSearchDraft } from "@/hooks"
import type { CommandResultSummary, ShortName } from "@/models"
import { DataTable, Pagination } from "./data-table"
import { Empty, Heading, Status, Time, duration } from "./detail-primitives"
import { Outcome, RunCounts, RunDetail } from "./run-detail"
import { Button } from "./ui/button"
import { Input } from "./ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./ui/select"

const pageSize = 15
export function History({
  row,
  runId,
  admin,
  logs,
  shortNames,
}: {
  row: TargetRow
  runId?: string
  admin: boolean
  logs: boolean
  shortNames: ShortName[]
}) {
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const command = params.get("command") ?? "all"
  const outcome = params.get("outcome") ?? "all"
  const [query, setQuery] = useSearchDraft(params.get("runSearch") ?? "")
  const listParams = useMemo(() => {
    const filtered = new URLSearchParams()
    for (const key of ["command", "outcome", "runSearch"])
      if (params.has(key)) filtered.set(key, params.get(key)!)
    return filtered.toString()
  }, [params])
  const listPath = `${historyPath(row)}${listParams ? `?${listParams}` : ""}`
  const data = useMemo(
    () =>
      row.runs.filter((run) => {
        if (
          command !== "all" &&
          (run.commandInfo?.command ?? "Unknown") !== command
        )
          return false
        if (outcome !== "all" && runOutcome(run) !== outcome) return false
        const q = query.trim().toLowerCase()
        return (
          !q ||
          [
            run.id,
            run.reconcileId,
            run.gitInfo?.commit,
            run.gitInfo?.ref && JSON.stringify(run.gitInfo.ref),
          ].some((s) => s?.toLowerCase().includes(q))
        )
      }),
    [row.runs, command, outcome, query]
  )
  const [pagination, setPagination] = useState<PaginationState>(() => ({
    pageSize,
    pageIndex: Math.floor(
      Math.max(
        0,
        data.findIndex((run) => run.id === runId)
      ) / pageSize
    ),
  }))
  const page = Math.min(
    pagination.pageIndex,
    Math.max(0, Math.ceil(data.length / pageSize) - 1)
  )
  const active = row.runs.find((run) => run.id === runId)
  const isInFilter = !runId || data.some((run) => run.id === runId)
  const neighbors = isInFilter ? data : row.runs
  const index = neighbors.findIndex((run) => run.id === runId)
  const runPath = useCallback(
    (id: string) => {
      const next = new URLSearchParams(listParams)
      for (const key of ["view", "full"])
        if (params.has(key)) next.set(key, params.get(key)!)
      return `${historyPath(row, id)}${next.size ? `?${next}` : ""}`
    },
    [row, listParams, params]
  )
  const columns = useMemo<ColumnDef<CommandResultSummary>[]>(
    () => [
      {
        id: "run",
        header: "Started / command",
        cell: ({ row: { original: run } }) => (
          <>
            <Link
              className="font-medium hover:underline"
              preventScrollReset
              aria-current={run.id === runId ? "page" : undefined}
              to={runPath(run.id)}
            >
              <Time value={run.commandInfo?.startTime} />
            </Link>
            <div className="mt-0.5 flex items-center gap-2 text-xs text-muted-foreground">
              {run.commandInfo?.command ?? "Unknown"}
              {run.commandInfo?.dryRun && <Status>Dry run</Status>}
              {row.runs[0]?.id === run.id && <span>Latest</span>}
            </div>
          </>
        ),
      },
      {
        id: "outcome",
        header: "Outcome",
        cell: ({ row: { original: run } }) => (
          <div className="space-y-1">
            <Outcome run={run} />
            <div>
              <RunCounts run={run} />
            </div>
          </div>
        ),
      },
      {
        id: "duration",
        header: "Duration",
        cell: ({ row: { original: run } }) =>
          duration(run.commandInfo?.startTime, run.commandInfo?.endTime),
      },
      {
        id: "commit",
        header: "Commit",
        cell: ({ row: { original: run } }) => (
          <span className="font-mono text-xs" title={run.gitInfo?.commit}>
            {run.gitInfo?.commit?.slice(0, 12) || "—"}
            {run.gitInfo?.dirty ? " (dirty)" : ""}
          </span>
        ),
      },
    ],
    [row, runId, runPath]
  )
  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack v8; React Compiler is not enabled.
  const table = useReactTable({
    data,
    columns,
    getRowId: (run) => run.id,
    state: { pagination: { pageIndex: page, pageSize } },
    onPaginationChange: setPagination,
    autoResetPageIndex: false,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })
  function filter(name: string, value: string) {
    if (name === "runSearch") setQuery(value)
    setPagination({ pageIndex: 0, pageSize })
    setParams(
      (previous) => {
        const next = new URLSearchParams(previous)
        const text = name === "runSearch" ? value : query
        if (text) next.set("runSearch", text)
        else next.delete("runSearch")
        if (name !== "runSearch") {
          if (value !== "all") next.set(name, value)
          else next.delete(name)
        }
        return next
      },
      { replace: true, preventScrollReset: true }
    )
  }
  function step(offset: number) {
    const next = neighbors[index + offset]
    if (!next) return
    if (isInFilter)
      setPagination({
        pageIndex: Math.floor((index + offset) / pageSize),
        pageSize,
      })
    navigate(runPath(next.id), { preventScrollReset: true })
  }
  if (!row.runs.length && !runId)
    return <Empty>No recorded runs for this target.</Empty>
  const commands = [
    ...new Set(row.runs.map((run) => run.commandInfo?.command ?? "Unknown")),
  ].sort()
  return (
    <div className="min-w-0">
      {!runId && (
        <section className="space-y-4" aria-label="Run history">
          <Heading title="Run history">
            Newest first · only retained results are shown.
          </Heading>
          <div className="flex flex-wrap gap-2">
            <Input
              className="w-48 max-w-full"
              aria-label="Search runs"
              placeholder="Commit, ref, or run ID…"
              value={query}
              onChange={(e) => filter("runSearch", e.target.value)}
            />
            <Select
              value={command}
              onValueChange={(value) => filter("command", value)}
            >
              <SelectTrigger aria-label="Command">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All commands</SelectItem>
                {commands.map((value) => (
                  <SelectItem value={value} key={value}>
                    {value}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={outcome}
              onValueChange={(value) => filter("outcome", value)}
            >
              <SelectTrigger aria-label="Outcome">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All outcomes</SelectItem>
                {["Errors", "Warnings", "Changes", "No changes"].map(
                  (value) => (
                    <SelectItem value={value} key={value}>
                      {value}
                    </SelectItem>
                  )
                )}
              </SelectContent>
            </Select>
          </div>
          <DataTable
            table={table}
            label="Recorded runs"
            active={runId}
            empty="No recorded runs match these filters."
            onOpen={(run) =>
              navigate(runPath(run.id), { preventScrollReset: true })
            }
          />
          {table.getPageCount() > 1 && <Pagination table={table} noun="runs" />}
        </section>
      )}
      {runId && (
        <section className="min-w-0 space-y-4" aria-label="Run details">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <Button asChild size="sm" variant="outline">
              <Link to={listPath} preventScrollReset>
                <List /> All runs
              </Link>
            </Button>
            <div className="flex flex-wrap gap-1">
              <Button
                variant="outline"
                size="sm"
                onClick={() => step(-1)}
                disabled={index <= 0}
              >
                <ChevronLeft /> Newer
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => step(1)}
                disabled={index < 0 || index >= neighbors.length - 1}
              >
                Older <ChevronRight />
              </Button>
            </div>
          </div>
          {active ? (
            <>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <Heading title={active.commandInfo?.command ?? "Recorded run"}>
                  <Time value={active.commandInfo?.startTime} /> ·{" "}
                  {duration(
                    active.commandInfo?.startTime,
                    active.commandInfo?.endTime
                  )}
                  {active.commandInfo?.dryRun ? " · Dry run" : ""}
                </Heading>
                <Outcome run={active} />
              </div>
              {!isInFilter && (
                <p className="text-sm text-muted-foreground">
                  This run is outside the current filters. Newer and older
                  browse all runs.
                </p>
              )}
              <RunDetail
                key={active.id}
                run={active}
                admin={admin}
                logs={logs}
                shortNames={shortNames}
              />
            </>
          ) : (
            <Empty>
              This run is not available. It may have expired or may not be
              included in this snapshot.
            </Empty>
          )}
        </section>
      )}
    </div>
  )
}

import { useEffect, useRef, useState } from "react"
import { ChevronRight } from "lucide-react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import { socketUrl } from "@/api"
import { refKey, refLabel } from "@/domain"
import type {
  DeploymentError,
  DriftedObject,
  ObjectRef,
  ResultObject,
  ValidateResult,
} from "@/models"
import { Code, Empty, Heading, Status, Time, Yaml } from "./detail-primitives"
import { Input } from "./ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./ui/select"

export function Messages({
  errors,
  warnings,
}: {
  errors?: DeploymentError[]
  warnings?: DeploymentError[]
}) {
  const messages = [
    ...(errors ?? []).map((value) => ({ ...value, kind: "Error" as const })),
    ...(warnings ?? []).map((value) => ({
      ...value,
      kind: "Warning" as const,
    })),
  ]
  if (!messages.length) return <Empty>No errors or warnings.</Empty>
  return (
    <ul aria-label="Result messages" className="divide-y rounded-lg border">
      {messages.map(({ kind, ref, message }, index) => (
        <li key={index} className="space-y-2 p-3 sm:p-4">
          <div className="flex flex-wrap items-center gap-2">
            <Status tone={kind === "Error" ? "bad" : "warn"}>{kind}</Status>
            <span
              className="text-sm font-medium wrap-anywhere"
              title={ref && [ref.group, ref.version].filter(Boolean).join("/")}
            >
              {refLabel(ref)}
            </span>
          </div>
          <p className="text-sm wrap-anywhere whitespace-pre-wrap text-muted-foreground">
            {message}
          </p>
        </li>
      ))}
    </ul>
  )
}

type Changed = DriftedObject | ResultObject
const categories = ["New", "Changed", "Deleted", "Orphaned"] as const
export function categoriesOf(object: Changed) {
  return categories.filter((category) =>
    category === "New"
      ? object.new
      : category === "Changed"
        ? object.changes?.length
        : category === "Deleted"
          ? object.deleted
          : object.orphan
  )
}
export function DiffView({
  object,
  showCategories = true,
}: {
  object: Changed
  showCategories?: boolean
}) {
  return (
    <div className="space-y-3">
      {showCategories && (
        <div className="flex flex-wrap items-center gap-2">
          {categoriesOf(object).map((category) => (
            <Status key={category}>{category}</Status>
          ))}
        </div>
      )}
      {object.changes?.length ? (
        object.changes.map((change, index) => (
          <div key={index} className="space-y-1.5">
            <h4 className="font-mono text-xs font-semibold break-all">
              {change.jsonPath || "Whole object"}
            </h4>
            {change.unifiedDiff ? (
              <Code text={change.unifiedDiff} diff />
            ) : (
              <Yaml
                value={{ before: change.oldValue, after: change.newValue }}
              />
            )}
          </div>
        ))
      ) : (
        <p className="text-sm text-muted-foreground">
          No field diff was recorded.
        </p>
      )}
    </div>
  )
}
export function Changes({
  objects,
  openResource,
}: {
  objects?: Changed[]
  openResource?: (ref: ObjectRef) => React.ReactNode
}) {
  const [filter, setFilter] = useState("all")
  const [query, setQuery] = useState("")
  const changed = (objects ?? []).filter(
    (object) => categoriesOf(object).length
  )
  if (!changed.length) return <Empty>No resource changes recorded.</Empty>
  const sorted = changed.filter(
    (object) =>
      (filter === "all" ||
        categoriesOf(object).some((category) => category === filter)) &&
      refLabel(object.ref).toLowerCase().includes(query.trim().toLowerCase())
  )
  return (
    <div className="space-y-3">
      {(changed.length > 3 || query || filter !== "all") && (
        <div className="flex flex-wrap gap-2">
          <Input
            aria-label="Search changed resources"
            placeholder="Find a resource…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="w-56"
          />
          <Select value={filter} onValueChange={setFilter}>
            <SelectTrigger aria-label="Change type">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                All changes ({changed.length})
              </SelectItem>
              {categories.map((category) => (
                <SelectItem key={category} value={category}>
                  {category} (
                  {
                    changed.filter((object) =>
                      categoriesOf(object).includes(category)
                    ).length
                  }
                  )
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
      {!sorted.length ? (
        <Empty>No resources match these filters.</Empty>
      ) : (
        <div className="divide-y rounded-lg border">
          {sorted.map((object) => (
            <details
              key={refKey(object.ref)}
              open={changed.length === 1 || undefined}
              className="group/change"
            >
              <summary className="recorded-toggle min-h-12 flex-wrap rounded-lg py-2.5">
                <ChevronRight
                  aria-hidden="true"
                  className="size-4 shrink-0 group-open/change:rotate-90"
                />
                <span
                  className="min-w-32 flex-1 text-sm font-medium wrap-anywhere text-foreground"
                  title={[object.ref.group, object.ref.version]
                    .filter(Boolean)
                    .join("/")}
                >
                  {refLabel(object.ref)}
                </span>
                <span className="flex flex-wrap gap-1.5">
                  {categoriesOf(object).map((category) => (
                    <Status key={category}>{category}</Status>
                  ))}
                </span>
              </summary>
              <div className="min-w-0 space-y-2 border-t px-3 py-3 sm:px-4">
                {openResource?.(object.ref)}
                <DiffView object={object} showCategories={false} />
              </div>
            </details>
          ))}
        </div>
      )}
    </div>
  )
}
export function ValidationDetails({ data }: { data: ValidateResult }) {
  return (
    <div className="space-y-4">
      <Heading title="Validation results">
        <Time value={data.startTime} /> · {data.ready ? "Ready" : "Not ready"}
      </Heading>
      <Messages errors={data.errors} warnings={data.warnings} />
      {!!data.results?.length && (
        <div className="divide-y border-y">
          {data.results.map((result, index) => (
            <section key={index} className="space-y-1 py-3">
              <h3 className="text-sm font-semibold">{refLabel(result.ref)}</h3>
              {result.annotation && (
                <p className="font-mono text-xs text-muted-foreground">
                  {result.annotation}
                </p>
              )}
              <div className="markdown text-sm">
                <ReactMarkdown
                  remarkPlugins={[remarkGfm]}
                  components={{
                    a: ({ children, href }) => (
                      <a href={href} target="_blank" rel="noreferrer">
                        {children}
                      </a>
                    ),
                  }}
                >
                  {result.message}
                </ReactMarkdown>
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  )
}

type LogEntry = {
  msg: string
  level?: string
  ts?: string | number
  name?: string
  namespace?: string
}
const maxLines = 2000
export function Logs({ query }: { query: Record<string, string> }) {
  const search = new URLSearchParams(query).toString()
  const [lines, setLines] = useState<LogEntry[]>([])
  const [status, setStatus] = useState("Connecting…")
  const [attempt, setAttempt] = useState(0)
  const [follow, setFollow] = useState(true)
  const area = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const ws = new WebSocket(
      socketUrl("api/logs", Object.fromEntries(new URLSearchParams(search)))
    )
    let active = true
    let batch: LogEntry[] = []
    let frame = 0
    ws.onopen = () => {
      if (active) setStatus("Watching controller logs")
    }
    ws.onclose = () => {
      if (active) setStatus("Log stream disconnected")
    }
    ws.onmessage = (event) => {
      try {
        const value = JSON.parse(event.data) as LogEntry
        if (typeof value.msg !== "string") return
        batch.push(value)
        if (!frame)
          frame = requestAnimationFrame(() => {
            const entries = batch
            batch = []
            frame = 0
            if (active)
              setLines((previous) => [...previous, ...entries].slice(-maxLines))
          })
      } catch {
        /* ignore non-log frames */
      }
    }
    return () => {
      active = false
      cancelAnimationFrame(frame)
      ws.close()
    }
  }, [search, attempt])
  useEffect(() => {
    if (follow && area.current)
      area.current.scrollTop = area.current.scrollHeight
  }, [lines, follow])
  return (
    <div className="space-y-3">
      <Heading title="Controller logs">
        This is a live stream of recent controller output, not an archive of the
        run.
      </Heading>
      <div className="flex flex-wrap gap-4 text-sm">
        <span role="status">{status}</span>
        <label className="flex gap-1">
          <input
            type="checkbox"
            checked={follow}
            onChange={(e) => setFollow(e.target.checked)}
          />{" "}
          Follow
        </label>
        {status.includes("disconnected") && (
          <button
            type="button"
            className="underline"
            onClick={() => {
              setStatus("Connecting…")
              setLines([])
              setAttempt((i) => i + 1)
            }}
          >
            Reconnect
          </button>
        )}
      </div>
      <div
        ref={area}
        tabIndex={0}
        aria-label="Controller output"
        className="max-h-[32rem] min-h-28 overflow-auto rounded border bg-muted/35 p-3 font-mono text-xs"
      >
        {!lines.length && (
          <p className="text-muted-foreground">No log entries received.</p>
        )}
        {lines.map((line, index) => (
          <div className="py-0.5 break-words whitespace-pre-wrap" key={index}>
            {line.level && (
              <span className="text-muted-foreground">[{line.level}] </span>
            )}
            {line.msg}
          </div>
        ))}
      </div>
    </div>
  )
}

import { Fragment, useState, type ReactNode } from "react"
import { Link, useLocation, useSearchParams } from "react-router-dom"
import { AlertCircle, Check, Copy } from "lucide-react"
import { stringify } from "yaml"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { cn } from "@/lib/utils"
import type { Manifest } from "@/domain"

export type Tone = "good" | "warn" | "bad" | "info" | "muted"
const tones: Record<Tone, string> = {
  good: "border-emerald-300/55 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-200",
  warn: "border-amber-300/60 bg-amber-50 text-amber-900 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200",
  bad: "border-red-300/60 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200",
  info: "border-blue-300/55 bg-blue-50 text-blue-800 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-200",
  muted: "border-border bg-muted/40 text-muted-foreground",
}
export function Status({
  tone = "muted",
  children,
}: {
  tone?: Tone
  children: ReactNode
}) {
  return (
    <span className="signal-badge" data-tone={tone}>
      {children}
    </span>
  )
}
export function ErrorNotice({
  children,
  title = "Something went wrong",
}: {
  children: ReactNode
  title?: string
}) {
  return (
    <Alert variant="destructive" role="alert">
      <AlertCircle />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription className="break-words whitespace-pre-wrap">
        {children}
      </AlertDescription>
    </Alert>
  )
}
export function Empty({ children }: { children: ReactNode }) {
  return (
    <p className="border-y py-6 text-sm text-muted-foreground">{children}</p>
  )
}
export function Heading({
  title,
  children,
}: {
  title: string
  children?: ReactNode
}) {
  return (
    <div className="space-y-1">
      <h2 className="text-lg font-semibold tracking-tight">{title}</h2>
      {children && (
        <div className="text-sm text-muted-foreground">{children}</div>
      )}
    </div>
  )
}
export function Properties({ items }: { items: [string, ReactNode][] }) {
  const present = items.filter(
    ([, value]) => value !== undefined && value !== null && value !== ""
  )
  return (
    <dl className="grid divide-y border-y text-sm">
      {present.map(([label, value]) => (
        <div
          key={label}
          className="grid gap-0.5 px-2 py-2 sm:grid-cols-[minmax(7rem,1fr)_minmax(0,3fr)] sm:gap-4"
        >
          <dt className="font-medium text-muted-foreground">{label}</dt>
          <dd className="min-w-0 wrap-anywhere">{value}</dd>
        </div>
      ))}
    </dl>
  )
}

const dateFormat = new Intl.DateTimeFormat(undefined, {
  dateStyle: "medium",
  timeStyle: "medium",
})
export function formatDate(value?: string) {
  return value &&
    Number.isFinite(Date.parse(value)) &&
    !value.startsWith("0001-")
    ? dateFormat.format(new Date(value))
    : "—"
}
export function Time({ value }: { value?: string }) {
  return (
    <time dateTime={value} title={value}>
      {formatDate(value)}
    </time>
  )
}
export function duration(start?: string, end?: string) {
  if (!start || !end || end.startsWith("0001-")) return "—"
  const elapsed = Date.parse(end) - Date.parse(start)
  if (!Number.isFinite(elapsed) || elapsed < 0) return "—"
  const seconds = Math.floor(elapsed / 1000)
  if (seconds >= 3600)
    return `${Math.floor(seconds / 3600)}h ${Math.floor(seconds / 60) % 60}m`
  if (seconds >= 60) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
  return seconds >= 1 ? `${seconds}s` : `${elapsed} ms`
}

export function Code({ text, diff = false }: { text: string; diff?: boolean }) {
  const [copied, setCopied] = useState(false)
  return (
    <div className="overflow-hidden rounded-md border bg-muted/30">
      {navigator.clipboard && (
        <div className="flex justify-end border-b px-1 py-0.5">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              navigator.clipboard
                .writeText(text)
                .then(() => setCopied(true))
                .catch(() => setCopied(false))
            }}
          >
            {copied ? <Check /> : <Copy />}
            {copied ? "Copied" : "Copy"}
          </Button>
        </div>
      )}
      <pre
        tabIndex={0}
        className="max-h-[32rem] overflow-auto py-2 text-xs leading-relaxed"
      >
        <code>
          {diff ? (
            text.split("\n").map((line, index) => (
              <span
                className={cn(
                  "block min-w-max px-3",
                  line.startsWith("+")
                    ? tones.good
                    : line.startsWith("-")
                      ? tones.bad
                      : line.startsWith("@@") &&
                        "text-blue-700 dark:text-blue-300"
                )}
                key={index}
              >
                {line || " "}
              </span>
            ))
          ) : (
            <span className="block px-3">{text}</span>
          )}
        </code>
      </pre>
    </div>
  )
}
export function Yaml({ value }: { value: unknown }) {
  return value === undefined || value === null ? (
    <Empty>Not available.</Empty>
  ) : (
    <Code text={stringify(value)} />
  )
}
export function ManifestView({
  manifest,
  initialStatus = false,
}: {
  manifest: Manifest
  initialStatus?: boolean
}) {
  const [showStatus, setShowStatus] = useState(initialStatus)
  const [showManaged, setShowManaged] = useState(false)
  const { status, metadata, ...rest } = manifest
  const { managedFields, ...meta } = metadata ?? {}
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-4 text-sm">
        {status != null && (
          <label className="flex items-center gap-1.5">
            <input
              type="checkbox"
              checked={showStatus}
              onChange={(e) => setShowStatus(e.target.checked)}
            />{" "}
            Show status
          </label>
        )}
        {managedFields != null && (
          <label className="flex items-center gap-1.5">
            <input
              type="checkbox"
              checked={showManaged}
              onChange={(e) => setShowManaged(e.target.checked)}
            />{" "}
            Show managed fields
          </label>
        )}
      </div>
      <Yaml
        value={{
          ...rest,
          ...(metadata && {
            metadata: {
              ...meta,
              ...(showManaged && managedFields != null && { managedFields }),
            },
          }),
          ...(showStatus && status != null && { status }),
        }}
      />
    </div>
  )
}

export function PageTabs({
  value,
  tabs,
}: {
  value: string
  tabs: { id: string; label: string; to: string; count?: number }[]
}) {
  return (
    <nav
      aria-label="Deployment sections"
      className="flex flex-wrap gap-x-4 border-t px-4 sm:gap-x-5 sm:px-5"
    >
      {tabs.map((tab) => (
        <Link
          to={tab.to}
          preventScrollReset
          key={tab.id}
          aria-current={value === tab.id ? "page" : undefined}
          className={cn(
            "inline-flex min-h-11 items-center gap-1.5 border-b-2 px-0.5 text-sm font-medium transition-colors focus-visible:-outline-offset-2",
            value === tab.id
              ? "border-primary text-primary"
              : "border-transparent text-muted-foreground hover:text-foreground"
          )}
        >
          {tab.label}
          {!!tab.count && (
            <span className="rounded bg-muted px-1.5 text-xs text-muted-foreground tabular-nums">
              {tab.count}
            </span>
          )}
        </Link>
      ))}
    </nav>
  )
}

export function QueryTabs({
  name,
  value,
  tabs,
}: {
  name: string
  value: string
  tabs: { id: string; label: string; content: ReactNode }[]
}) {
  const [, setParams] = useSearchParams()
  const location = useLocation()
  return (
    <Tabs
      value={value}
      onValueChange={(value) =>
        setParams(
          (previous) => {
            const next = new URLSearchParams(previous)
            next.set(name, value)
            if (name === "view") next.delete("full")
            return next
          },
          { preventScrollReset: true, state: location.state }
        )
      }
      className="min-w-0 gap-4"
    >
      <div className="overflow-x-auto">
        <TabsList>
          {tabs.map((tab) => (
            <TabsTrigger key={tab.id} value={tab.id}>
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </div>
      {tabs.map((tab) => (
        <Fragment key={tab.id}>{tab.content}</Fragment>
      ))}
    </Tabs>
  )
}

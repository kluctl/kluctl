import {
  CircleAlert,
  CircleCheck,
  Clock3,
  FlaskConical,
  GitCompareArrows,
  Hand,
  History,
  Pause,
  RefreshCw,
  TriangleAlert,
} from "lucide-react"
import { driftOf, type TargetRow } from "@/domain"
import { targetIndicators, type Indicator } from "@/inventory-view"

const indicators = {
  error: { icon: CircleAlert, label: "Error", tone: "bad" },
  warning: { icon: TriangleAlert, label: "Warnings", tone: "warn" },
  drift: { icon: GitCompareArrows, label: "Drift", tone: "drift" },
  approval: { icon: Hand, label: "Approval needed", tone: "warn" },
  ready: { icon: CircleCheck, label: "Ready", tone: "good" },
  suspended: { icon: Pause, label: "Suspended", tone: "muted" },
  running: { icon: RefreshCw, label: "Reconciling", tone: "info" },
  queued: { icon: Clock3, label: "Requested", tone: "info" },
  pending: { icon: Clock3, label: "Pending", tone: "muted" },
  dryRun: { icon: FlaskConical, label: "Dry run", tone: "muted" },
  recorded: { icon: History, label: "Recorded", tone: "muted" },
}

export function IndicatorBadge({
  indicator,
  count,
}: {
  indicator: Indicator
  count?: number
}) {
  const { icon: Icon, label, tone } = indicators[indicator]
  return (
    <span className="signal-badge" data-tone={tone}>
      <Icon
        aria-hidden="true"
        className={
          indicator === "running" ? "motion-safe:animate-spin" : undefined
        }
      />
      {label}
      {!!count && (
        <span className="signal-count">
          {count}
          <span className="sr-only">
            {" "}
            {count === 1 ? "resource" : "resources"}
          </span>
        </span>
      )}
    </span>
  )
}

export function TargetIndicators({ row }: { row: TargetRow }) {
  return (
    <span className="flex flex-wrap items-center gap-1.5">
      {targetIndicators(row).map((indicator) => (
        <IndicatorBadge
          key={indicator}
          indicator={indicator}
          count={
            indicator === "drift" ? driftOf(row)?.objects?.length : undefined
          }
        />
      ))}
    </span>
  )
}

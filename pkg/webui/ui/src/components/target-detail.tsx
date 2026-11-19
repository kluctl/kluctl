import { useCallback, type ReactNode } from "react"
import {
  Activity,
  ArrowRight,
  Box,
  ChevronRight,
  ClipboardCheck,
  GitCompareArrows,
  History as HistoryIcon,
  type LucideIcon,
} from "lucide-react"
import { Link } from "react-router-dom"
import { getValidation, snapshot } from "@/api"
import {
  approvalOf,
  deploymentSource,
  driftOf,
  historyPath,
  reconciliation,
  requestKinds,
  requestState,
  targetPath,
  validationOf,
  validationStatus,
  type TargetRow,
} from "@/domain"
import { useLoad } from "@/hooks"
import { deploymentName, projectIdentity, quantity } from "@/inventory-view"
import type { ShortName } from "@/models"
import { Changes, Logs, Messages, ValidationDetails } from "./diagnostics"
import {
  Empty,
  ErrorNotice,
  Heading,
  ManifestView,
  PageTabs,
  Properties,
  Status,
  Time,
  type Tone,
} from "./detail-primitives"
import { History } from "./history"
import { Outcome } from "./run-detail"
import { TargetActions } from "./target-actions"
import { IndicatorBadge, TargetIndicators } from "./target-indicators"

function CheckRow({
  icon: Icon,
  title,
  description,
  status,
  to,
}: {
  icon: LucideIcon
  title: string
  description: ReactNode
  status: ReactNode
  to?: string
}) {
  const content = (
    <>
      <span className="flex min-w-0 items-start gap-3">
        <span className="project-icon">
          <Icon aria-hidden="true" />
        </span>
        <span className="min-w-0">
          <span className="block text-sm font-semibold">{title}</span>
          <span className="mt-0.5 block text-[0.8125rem] wrap-anywhere text-muted-foreground">
            {description}
          </span>
        </span>
      </span>
      <span className="flex shrink-0 items-center gap-2">
        {status}
        <ChevronRight
          aria-hidden="true"
          className={to ? "size-4 text-muted-foreground" : "invisible size-4"}
        />
      </span>
    </>
  )
  const className =
    "grid min-h-16 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 px-4 py-3 sm:px-5"
  return to ? (
    <Link
      to={to}
      className={`${className} transition-colors hover:bg-muted/60 focus-visible:-outline-offset-2`}
    >
      {content}
    </Link>
  ) : (
    <div className={className}>{content}</div>
  )
}

const hasValidationDetails = (row: TargetRow) =>
  !!(
    (!snapshot && row.validation) ||
    row.deployment?.status?.lastValidateResult
  )

function CurrentState({ row }: { row: TargetRow }) {
  const state = reconciliation(row)
  const drift = driftOf(row)
  const driftCount = drift?.objects?.length ?? 0
  const validation = validationStatus(row)
  const check = validationOf(row)
  const prepareError = row.deployment?.status?.lastPrepareError
  const path = targetPath(row)
  const approval = approvalOf(row)
  const reconcileMessage = prepareError
    ? "The controller could not prepare this deployment."
    : state.tone === "bad"
      ? state.condition?.message ||
        "Reconciliation did not complete successfully."
      : state.label === "Suspended"
        ? "Automatic reconciliation is paused."
        : approval?.required
          ? "Review the drift before approving this deployment."
          : state.label === "Requested"
            ? "Waiting for the controller to start."
            : state.label === "Reconciling"
              ? state.condition?.message ||
                "The controller is working on this deployment."
              : row.deployment?.spec.dryRun
                ? "Dry run: the controller does not apply changes."
                : state.label === "Ready"
                  ? "The controller reconciled this deployment."
                  : "Waiting for the first controller result."
  const validationMessage = !check ? (
    validation.label === "Off" ? (
      "Automatic validation is disabled."
    ) : (
      "No validation result yet."
    )
  ) : (
    <>
      {check.errors ? `${quantity(check.errors, "error")} · ` : ""}
      {check.warnings
        ? `${quantity(check.warnings, "warning")} · `
        : ""}Checked <Time value={check.startTime} />
    </>
  )
  const driftMessages = drift?.errors?.length ? (
    <Status tone="bad">Check failed</Status>
  ) : drift?.warnings?.length ? (
    <Status tone="warn">Warnings</Status>
  ) : (
    <Status tone={drift ? "good" : "muted"}>
      {drift ? "In sync" : "Not checked"}
    </Status>
  )
  const validationTo = hasValidationDetails(row)
    ? `${path}/validation`
    : undefined
  return (
    <section className="content-panel overflow-hidden">
      <div className="border-b px-4 py-3.5 sm:px-5">
        <Heading title="Current state">
          {snapshot
            ? "Controller state at export time."
            : "Reported by the GitOps controller."}
        </Heading>
      </div>
      {prepareError && (
        <div className="border-b px-4 py-3 sm:px-5">
          <ErrorNotice title="Preparation failed">
            <span
              tabIndex={prepareError.length > 240 ? 0 : undefined}
              className="block max-h-40 overflow-auto wrap-anywhere"
            >
              {prepareError}
            </span>
          </ErrorNotice>
        </div>
      )}
      <div className="divide-y">
        <CheckRow
          icon={Activity}
          title="Reconciliation"
          description={reconcileMessage}
          status={
            approval?.required && state.label === "Unknown" ? (
              <IndicatorBadge indicator="approval" />
            ) : (
              <Status tone={state.tone}>
                {state.label === "Unknown" ? "Pending" : state.label}
              </Status>
            )
          }
        />
        <CheckRow
          icon={GitCompareArrows}
          title="Drift"
          to={`${path}/drift`}
          description={
            !drift
              ? "No comparison is available yet."
              : driftCount
                ? `${quantity(driftCount, "resource")} ${driftCount === 1 ? "differs" : "differ"} from the desired state.`
                : drift.errors?.length || drift.warnings?.length
                  ? "The comparison reported messages."
                  : "No differences from the desired state."
          }
          status={
            driftCount ? (
              <IndicatorBadge indicator="drift" count={driftCount} />
            ) : (
              driftMessages
            )
          }
        />
        <CheckRow
          icon={ClipboardCheck}
          title="Validation"
          to={validationTo}
          description={validationMessage}
          status={
            <Status tone={validation.tone}>
              {validation.label === "Off" ? "Disabled" : validation.label}
            </Status>
          }
        />
      </div>
    </section>
  )
}

function LatestRun({ row }: { row: TargetRow }) {
  const latest = row.runs[0]
  const report = latest ?? row.deployment?.status?.lastDeployResult
  const hasMessages = !!(report?.errors?.length || report?.warnings?.length)
  return (
    <section className="content-panel overflow-hidden">
      <div className="flex items-center justify-between gap-2 border-b px-4 py-3.5 sm:px-5">
        <h2 className="text-lg font-semibold tracking-tight">
          Latest recorded run
        </h2>
        {latest && (
          <Link
            className="shrink-0 text-xs font-medium text-primary underline-offset-4 hover:underline"
            to={historyPath(row)}
          >
            All runs
          </Link>
        )}
      </div>
      {latest ? (
        <CheckRow
          icon={HistoryIcon}
          title={`${latest.commandInfo?.command ?? "Run"}${latest.commandInfo?.dryRun ? " (dry run)" : ""}`}
          description={<Time value={latest.commandInfo?.startTime} />}
          status={<Outcome run={latest} />}
          to={`${historyPath(row, latest.id)}${hasMessages ? "?view=messages" : ""}`}
        />
      ) : report ? (
        <>
          <CheckRow
            icon={HistoryIcon}
            title={report.commandInfo?.command ?? "Deploy"}
            description={
              <>
                {" "}
                <Time value={report.commandInfo?.startTime} /> · Full result not
                retained
              </>
            }
            status={<Outcome run={report} />}
          />
          {hasMessages && (
            <div className="px-4 pb-4 sm:px-5">
              <Messages errors={report.errors} warnings={report.warnings} />
            </div>
          )}
        </>
      ) : (
        <p className="px-4 py-4 text-sm text-muted-foreground sm:px-5">
          No runs recorded for this target.
        </p>
      )}
    </section>
  )
}

function Context({ row }: { row: TargetRow }) {
  const deployment = row.deployment
  const project = projectIdentity(row)
  const ref = deployment && deploymentSource(deployment)?.ref
  const refText = Object.values(ref ?? {})
    .filter(Boolean)
    .join(" · ")
  const state = reconciliation(row)
  const targetCluster = row.target.clusterId
  const controllerCluster = row.instance?.clusterId
  const items: [string, ReactNode][] = [
    ["Project", project.location || project.name],
    ["Source ref", refText],
    [
      "Target",
      row.target.targetName || (targetCluster ? "Default" : "Not resolved"),
    ],
    ["Namespace", row.instance?.namespace],
    [
      "Target cluster",
      targetCluster ? (
        <code className="text-xs">{targetCluster}</code>
      ) : (
        "Not resolved"
      ),
    ],
    [
      "Controller cluster",
      controllerCluster && controllerCluster !== targetCluster ? (
        <code className="text-xs">{controllerCluster}</code>
      ) : undefined,
    ],
  ]
  const technical: [string, ReactNode][] = [
    ["Discriminator", row.target.discriminator],
    ["Condition", state.condition?.reason],
    ["Message", state.condition?.message],
    [
      "Last transition",
      state.condition?.lastTransitionTime && (
        <Time value={state.condition.lastTransitionTime} />
      ),
    ],
    ["Observed object hash", deployment?.status?.lastObjectsHash],
    ["Approved object hash", deployment?.spec.manualObjectsHash],
  ]
  return (
    <section className="content-panel px-4 py-3.5 sm:px-5">
      <Heading title={deployment ? "Deployment details" : "Target details"} />
      <dl className="mt-4 grid gap-x-5 gap-y-2 text-sm sm:grid-cols-[auto_minmax(0,1fr)]">
        {items
          .filter(([, value]) => value != null && value !== "")
          .map(([label, value]) => (
            <div
              key={label}
              className="grid grid-cols-[minmax(6.25rem,auto)_minmax(0,1fr)] gap-x-3 sm:contents"
            >
              <dt className="text-muted-foreground">{label}</dt>
              <dd className="min-w-0 font-medium wrap-anywhere">{value}</dd>
            </div>
          ))}
      </dl>
      {technical.some(([, value]) => value != null && value !== "") && (
        <details className="group/technical mt-4 border-t pt-2">
          <summary className="recorded-toggle -mx-2 px-2 sm:-mx-2 sm:px-2">
            <ChevronRight
              aria-hidden="true"
              className="size-3.5 group-open/technical:rotate-90"
            />
            Technical details
          </summary>
          <Properties items={technical} />
        </details>
      )}
      {deployment && (
        <Link
          className="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-primary underline-offset-4 hover:underline"
          to={`${targetPath(row)}/manifest`}
        >
          View configuration{" "}
          <ArrowRight aria-hidden="true" className="size-3.5" />
        </Link>
      )}
    </section>
  )
}

function ControllerRequests({ row }: { row: TargetRow }) {
  if (!row.deployment) return null
  const deploy = row.deployment
  const requests = requestKinds
    .map((kind) => ({ kind, ...requestState(deploy, kind) }))
    .filter(({ state }) => state !== "Not requested")
  const active = requests.filter(
    ({ state }) => state === "Requested" || state === "Running"
  ).length
  const hasFailure = requests.some(({ state }) => state === "Failed")
  const tone = (state: string): Tone =>
    state === "Failed" ? "bad" : state === "Completed" ? "good" : "info"
  return (
    <details className="content-panel group/requests">
      <summary className="recorded-toggle min-h-12 text-sm text-foreground">
        <ChevronRight
          aria-hidden="true"
          className="size-4 group-open/requests:rotate-90"
        />
        Controller requests
        {active ? (
          <Status tone="info">{active} active</Status>
        ) : hasFailure ? (
          <Status tone="bad">Failed request</Status>
        ) : null}
      </summary>
      <div className="px-4 pt-1 pb-4 sm:px-5">
        {requests.length ? (
          <Properties
            items={requests.map(({ kind, state, result }) => {
              const link =
                state !== "Requested" &&
                result?.resultId &&
                (kind === "validate"
                  ? !snapshot &&
                    result.resultId === row.validation?.id &&
                    `${targetPath(row)}/validation`
                  : row.runs.some((run) => run.id === result.resultId) &&
                    historyPath(row, result.resultId))
              return [
                kind[0].toUpperCase() + kind.slice(1),
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <Status tone={tone(state)}>{state}</Status>
                    {state !== "Requested" && result?.endTime && (
                      <Time value={result.endTime} />
                    )}
                    {link && (
                      <Link to={link} className="text-primary underline">
                        {kind === "validate" ? "View validation" : "View run"}
                      </Link>
                    )}
                  </div>
                  {state === "Failed" && result?.commandError && (
                    <p className="wrap-anywhere text-destructive">
                      {result.commandError}
                    </p>
                  )}
                </div>,
              ]
            })}
          />
        ) : (
          <p className="text-sm text-muted-foreground">
            No controller requests recorded.
          </p>
        )}
      </div>
    </details>
  )
}

function TargetSummary({ row }: { row: TargetRow }) {
  return (
    <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
      <div className="min-w-0 space-y-4">
        {row.deployment && <CurrentState row={row} />}
        {!row.deployment && (
          <p className="content-panel px-4 py-3 text-sm text-muted-foreground">
            This target shows recorded runs, not a live deployment.
          </p>
        )}
        <LatestRun row={row} />
        {row.deployment && <ControllerRequests row={row} />}
      </div>
      <Context row={row} />
    </div>
  )
}

function Drift({ row }: { row: TargetRow }) {
  const drift = driftOf(row)
  if (!drift)
    return (
      <Empty>
        {row.deployment?.status?.lastDriftDetectionResultMessage ||
          "No drift check has been recorded."}
      </Empty>
    )
  const count = drift.objects?.length ?? 0
  const messages = !!(drift.errors?.length || drift.warnings?.length)
  return (
    <div className="space-y-4">
      <Heading
        title={
          count
            ? `${quantity(count, "resource")} out of sync`
            : messages
              ? "Drift check messages"
              : "No drift detected"
        }
      >
        Desired state compared with the cluster · Checked{" "}
        <Time value={drift.endTime || drift.startTime} />
      </Heading>
      {messages && <Messages errors={drift.errors} warnings={drift.warnings} />}
      {count > 0 && <Changes objects={drift.objects} />}
    </div>
  )
}
function Validation({ id }: { id: string }) {
  const load = useCallback(
    (signal: AbortSignal) => getValidation(id, signal),
    [id]
  )
  const state = useLoad(id, load)
  return state?.error ? (
    <ErrorNotice title="Could not load validation">{state.error}</ErrorNotice>
  ) : state?.value ? (
    <ValidationDetails data={state.value} />
  ) : (
    <p role="status" className="text-sm text-muted-foreground">
      Loading validation…
    </p>
  )
}

export function TargetDetail({
  row,
  inventoryPath,
  section,
  runId,
  admin,
  logs,
  shortNames,
}: {
  row: TargetRow
  inventoryPath: string
  section: string
  runId?: string
  admin: boolean
  logs: boolean
  shortNames: ShortName[]
}) {
  const path = targetPath(row)
  const project = projectIdentity(row)
  const count = driftOf(row)?.objects?.length ?? 0
  const tabs: { id: string; label: string; to: string; count?: number }[] = [
    { id: "summary", label: "Overview", to: path },
  ]
  if (row.deployment)
    tabs.push({ id: "drift", label: "Drift", to: `${path}/drift`, count })
  if (hasValidationDetails(row))
    tabs.push({
      id: "validation",
      label: "Validation",
      to: `${path}/validation`,
    })
  tabs.push({
    id: "results",
    label: "Run history",
    to: historyPath(row),
    count: row.runs.length,
  })
  if (row.deployment)
    tabs.push({ id: "manifest", label: "Manifest", to: `${path}/manifest` })
  if (logs && row.deployment)
    tabs.push({ id: "logs", label: "Logs", to: `${path}/logs` })
  const active = tabs.some((tab) => tab.id === section) ? section : "summary"
  return (
    <section className="space-y-4">
      <nav
        aria-label="Breadcrumb"
        className="flex min-w-0 items-center gap-2 text-[0.8125rem] text-muted-foreground"
      >
        <Link
          to={inventoryPath}
          className="font-medium hover:text-primary hover:underline"
        >
          Projects
        </Link>
        <ChevronRight aria-hidden="true" className="size-3.5 shrink-0" />
        <span title={project.location || project.name} className="truncate">
          {project.name}
        </span>
      </nav>
      <div className="content-panel">
        <div className="flex flex-col gap-4 px-4 py-5 sm:flex-row sm:justify-between sm:px-5">
          <div className="flex min-w-0 items-start gap-3.5">
            <span className="project-icon size-11 sm:size-12">
              <Box aria-hidden="true" className="size-6" />
            </span>
            <div className="min-w-0">
              <h1 className="text-[1.7rem] leading-tight font-semibold tracking-tight wrap-anywhere sm:text-3xl">
                {deploymentName(row)}
              </h1>
              <p className="mt-0.5 text-sm wrap-anywhere text-muted-foreground">
                {row.deployment
                  ? `GitOps · ${row.instance?.namespace}`
                  : row.instance
                    ? `Historical · ${row.instance.namespace}`
                    : "CLI"}
                {row.target.targetName && (
                  <> · Target: {row.target.targetName}</>
                )}
                {row.deployment?.spec.dryRun && <> · Dry run</>}
              </p>
              <div className="mt-2.5">
                <TargetIndicators row={row} />
              </div>
            </div>
          </div>
          {admin && !snapshot && row.deployment && row.instance && (
            <TargetActions row={row} />
          )}
        </div>
        <PageTabs value={active} tabs={tabs} />
      </div>
      {active === "summary" ? (
        <TargetSummary row={row} />
      ) : (
        <div className="content-panel p-4 sm:p-5">
          {active === "results" && (
            <History
              row={row}
              runId={runId}
              admin={admin}
              logs={logs}
              shortNames={shortNames}
            />
          )}
          {active === "drift" && <Drift row={row} />}
          {active === "validation" &&
            (!snapshot && row.validation ? (
              <Validation key={row.validation.id} id={row.validation.id} />
            ) : (
              row.deployment?.status?.lastValidateResult && (
                <ValidationDetails
                  data={row.deployment.status.lastValidateResult}
                />
              )
            ))}
          {active === "manifest" && row.deployment && (
            <div className="space-y-4">
              <Heading title="KluctlDeployment manifest">
                Configuration reported by Kubernetes. This view is read-only.
              </Heading>
              <ManifestView manifest={row.deployment} />
            </div>
          )}
          {active === "logs" && row.instance && (
            <Logs
              query={{
                cluster: row.instance.clusterId,
                namespace: row.instance.namespace,
                name: row.instance.name,
              }}
            />
          )}
        </div>
      )}
    </section>
  )
}

import { useState } from "react"
import {
  Check,
  ChevronDown,
  CircleCheck,
  Pause,
  Play,
  RefreshCw,
  Rocket,
  Trash2,
} from "lucide-react"
import { post } from "@/api"
import {
  approvalOf,
  requestState,
  type RequestKind,
  type TargetRow,
} from "@/domain"
import { ErrorNotice } from "./detail-primitives"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "./ui/alert-dialog"
import { Button } from "./ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu"

const actions = [
  {
    kind: "reconcile",
    label: "Reconcile",
    icon: RefreshCw,
    endpoint: "reconcileNow",
    description: "Request a full reconciliation of this deployment.",
  },
  {
    kind: "validate",
    label: "Validate",
    icon: CircleCheck,
    endpoint: "validateNow",
    description: "Request a validation of this deployment.",
  },
  {
    kind: "deploy",
    label: "Deploy",
    icon: Rocket,
    endpoint: "deployNow",
    description: "Request a deployment using the configured deploy mode.",
  },
  {
    kind: "prune",
    label: "Prune",
    icon: Trash2,
    endpoint: "pruneNow",
    description:
      "Request deletion of resources that no longer belong to this deployment's desired state.",
    confirm: true,
  },
] as const

type Action = {
  label: string
  endpoint: string
  description: string
  confirm?: boolean
  fields?: Record<string, unknown>
}

export function TargetActions({ row }: { row: TargetRow }) {
  const [pending, setPending] = useState(false)
  const [confirmation, setConfirmation] = useState<Action>()
  const [feedback, setFeedback] = useState<{ value: string; error?: boolean }>()
  const { deployment, instance } = row
  if (!deployment || !instance) return null
  const approval = approvalOf(row)
  const qualifiedName = `${instance.namespace}/${instance.name}`
  const manual: Action | undefined =
    approval &&
    (approval.required
      ? {
          label: "Approve deployment",
          endpoint: "setManualObjectsHash",
          description: `Approve the rendered object hash ${approval.hash} for deployment.`,
          fields: { objectsHash: approval.hash },
          confirm: true,
        }
      : {
          ...actions[2],
          label: "Deploy again",
          confirm: true,
        })
  const suspended = deployment.spec.suspend
  const suspend: Action = suspended
    ? {
        label: "Resume",
        endpoint: "setSuspended",
        description: "Resume automatic reconciliation.",
        fields: { suspend: false },
      }
    : {
        label: "Suspend",
        endpoint: "setSuspended",
        description:
          "Pause automatic reconciliation. This does not stop work already in progress.",
        fields: { suspend: true },
        confirm: true,
      }
  function busy(kind: RequestKind) {
    const state = requestState(deployment!, kind).state
    return state === "Requested" || state === "Running"
  }
  async function execute(action: Action) {
    if (!instance) return
    setConfirmation(undefined)
    if (
      action.endpoint === "setManualObjectsHash" &&
      action.fields?.objectsHash !== approval?.hash
    ) {
      setFeedback({
        value:
          "The rendered state changed. Review the latest drift before approving.",
        error: true,
      })
      return
    }
    setPending(true)
    setFeedback(undefined)
    try {
      await post(`api/${action.endpoint}`, {
        cluster: instance.clusterId,
        name: instance.name,
        namespace: instance.namespace,
        ...action.fields,
      })
      const updatesSpec =
        action.endpoint === "setSuspended" ||
        action.endpoint === "setManualObjectsHash"
      setFeedback({
        value: updatesSpec
          ? `${action.label} saved. Waiting for the controller to report the new state.`
          : `${action.label} request sent. Check Controller requests for progress.`,
      })
    } catch (error) {
      setFeedback({ value: (error as Error).message, error: true })
    } finally {
      setPending(false)
    }
  }
  function choose(action: Action) {
    if (action.confirm) setConfirmation(action)
    else void execute(action)
  }
  return (
    <div className="min-w-0 space-y-2 sm:ml-auto">
      <div className="flex flex-wrap gap-2 sm:justify-end">
        {manual && (
          <Button
            className="h-9"
            disabled={pending || busy("deploy")}
            onClick={() => choose(manual)}
          >
            <Play aria-hidden="true" /> {manual.label}
          </Button>
        )}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button className="h-9" variant="outline" disabled={pending}>
              {pending ? "Sending…" : "Actions"}{" "}
              <ChevronDown aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="min-w-36">
            {actions.map((action) => (
              <DropdownMenuItem
                key={action.endpoint}
                disabled={busy(action.kind)}
                onSelect={() => choose(action)}
              >
                <action.icon aria-hidden="true" /> {action.label}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => choose(suspend)}>
              {suspended ? (
                <Play aria-hidden="true" />
              ) : (
                <Pause aria-hidden="true" />
              )}{" "}
              {suspend.label}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {feedback &&
        (feedback.error ? (
          <ErrorNotice title="Could not send the request">
            {feedback.value}
          </ErrorNotice>
        ) : (
          <p
            role="status"
            className="flex max-w-72 items-start gap-1.5 text-xs text-muted-foreground"
          >
            <Check
              aria-hidden="true"
              className="size-3.5 shrink-0 text-primary"
            />{" "}
            {feedback.value}
          </p>
        ))}
      <AlertDialog
        open={!!confirmation}
        onOpenChange={(open) => {
          if (!open) setConfirmation(undefined)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="wrap-anywhere">
              {confirmation?.label} {qualifiedName}?
            </AlertDialogTitle>
            <AlertDialogDescription className="wrap-anywhere">
              {confirmation?.description} The request will be sent to cluster{" "}
              {instance.clusterId}.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (confirmation) void execute(confirmation)
              }}
            >
              {confirmation?.label ?? "Confirm"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

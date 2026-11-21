import { describe, expect, it } from "vitest"
import type { CommandResultSummary, ValidateResultSummary } from "./models"
import {
  applyEvents,
  deploymentId,
  emptyInventory,
  findLinkedTarget,
  findTargetById,
  reconciliation,
  requestState,
  targetId,
  targetSignals,
  targetsFrom,
  type Deployment,
  type Event,
} from "./domain"

const project = { repoKey: "git://example.com/shop", subDir: "env" }
const target = {
  targetName: "prod",
  clusterId: "workload",
  discriminator: "shop-prod",
}
const instance = { clusterId: "management", namespace: "apps", name: "shop" }
const deployment: Deployment = {
  metadata: { uid: "one", name: "shop", namespace: "apps" },
  spec: { validate: true },
  status: { projectKey: project, targetKey: target },
}
function run(id: string, startTime: string, managed = false) {
  return {
    type: "update_command_result_summary",
    summary: {
      id,
      projectKey: project,
      targetKey: target,
      commandInfo: { command: "deploy", startTime },
      kluctlDeployment: managed ? instance : undefined,
    } as CommandResultSummary,
  } satisfies Event
}
function validation(id: string, startTime: string) {
  return {
    type: "update_validate_result_summary",
    summary: {
      id,
      projectKey: project,
      targetKey: target,
      kluctlDeployment: instance,
      startTime,
      warnings: 1,
    } as ValidateResultSummary,
  } satisfies Event
}

const crEvent = {
  type: "update_kluctl_deployment",
  clusterId: instance.clusterId,
  deployment,
} satisfies Event

describe("deployment inventory", () => {
  it("groups CLI runs separately from managed targets and orders historical runs newest first", () => {
    const rows = targetsFrom(
      applyEvents(emptyInventory(), [
        crEvent,
        run("old", "2025-02-01T10:00:00Z", true),
        run("cli", "2025-02-01T11:00:00Z"),
        run("recent", "2025-02-01T12:00:00Z", true),
        validation("newest", "2025-02-01T12:00:00Z"),
        validation("older", "2025-02-01T11:00:00Z"),
      ])
    )
    expect(rows).toHaveLength(2)
    const managed = rows.find((row) => row.instance)!
    expect(managed.instance?.clusterId).toEqual("management")
    expect(managed.target.clusterId).toEqual("workload")
    expect(managed.runs.map((run) => run.id)).toEqual(["recent", "old"])
    expect(managed.validation?.id).toEqual("newest")
    expect(targetSignals(managed).warnings).toEqual(1)
    expect(
      rows.find((row) => !row.instance)?.runs.map((run) => run.id)
    ).toEqual(["cli"])
  })

  it("shows a CR without runs and drops managed runs when its CR goes away", () => {
    const inventory = applyEvents(emptyInventory(), [
      crEvent,
      validation("check", "2025-02-01T12:00:00Z"),
    ])
    expect(targetsFrom(inventory)).toHaveLength(1)
    const updated = applyEvents(inventory, [
      run("managed", "2025-02-01T10:00:00Z", true),
      run("cli", "2025-02-01T10:00:00Z"),
    ])
    const rows = targetsFrom(
      applyEvents(updated, [
        { type: "delete_kluctl_deployment", id: deployment.metadata.uid },
      ])
    )
    expect(rows.map((row) => row.runs[0]?.id)).toEqual(["cli"])
  })

  it("keeps unresolved deployments and prepare errors visible before a target key exists", () => {
    const notResolved: Deployment = {
      ...deployment,
      spec: {
        ...deployment.spec,
        source: { git: { url: "git@example.com:shop.git", path: "./env/" } },
        target: "prod",
      },
      status: {
        lastPrepareError: "Target could not be resolved",
        conditions: [
          { type: "Ready", status: "False", reason: "PrepareFailed" },
        ],
      },
    }
    const inventory = applyEvents(emptyInventory(), [
      { ...crEvent, deployment: notResolved },
    ])
    const [row] = targetsFrom(inventory)
    expect(row.id).toBe(deploymentId(instance))
    expect(row.project).toEqual(project)
    expect(row.target).toEqual({ clusterId: "", targetName: "prod" })
    expect(reconciliation(row).label).toBe("Prepare failed")
    expect(targetSignals(row).errors).toBeGreaterThan(0)

    const ready = targetsFrom(applyEvents(inventory, [crEvent]))
    expect(findTargetById(ready, row.id)?.id).toBe(
      targetId(project, target, instance)
    )
  })

  it("prefers the live deployment when a legacy link also matches an older target", () => {
    const historical = run("old", "2025-02-01T10:00:00Z", true)
    historical.summary.targetKey = {
      ...target,
      targetName: "old-prod",
      discriminator: "old",
    }
    const rows = targetsFrom(
      applyEvents(emptyInventory(), [crEvent, historical])
    )
    expect(rows).toHaveLength(2)
    expect(
      findLinkedTarget(
        rows,
        new URLSearchParams({ kluctlDeploymentName: "shop" })
      )
    ).toBe(`/targets/${targetId(project, target, instance)}`)
    expect(
      findLinkedTarget(rows, new URLSearchParams({ commandResultId: "old" }))
    ).toBe(
      `/targets/${targetId(project, historical.summary.targetKey, instance)}/results/old`
    )
  })

  it("keeps target and run links stable independently of property order", () => {
    const id = targetId(project, target, instance)
    expect(id).toBe(
      "21dd81eccaed210441e538a4c526c75f1615feb6a70561ff47e2159cd44c2ef1"
    )
    expect(
      targetId(
        { ...project },
        {
          clusterId: "workload",
          discriminator: "shop-prod",
          targetName: "prod",
        },
        instance
      )
    ).toEqual(id)
    const rows = targetsFrom(
      applyEvents(emptyInventory(), [
        crEvent,
        run("saved", "2025-02-01T10:00:00Z", true),
      ])
    )
    expect(
      findLinkedTarget(rows, new URLSearchParams({ commandResultId: "saved" }))
    ).toEqual(`/targets/${id}/results/saved`)
    expect(
      findLinkedTarget(
        rows,
        new URLSearchParams({
          kluctlDeploymentClusterId: "management",
          targetClusterId: "workload",
        })
      )
    ).toEqual(`/targets/${id}`)
    expect(
      findLinkedTarget(
        rows,
        new URLSearchParams({ targetClusterId: "management" })
      )
    ).toBeUndefined()
  })
})

describe("controller requests", () => {
  it("does not confuse the currently queued request with an older completed request", () => {
    const updated = {
      ...deployment,
      metadata: {
        ...deployment.metadata,
        annotations: { "kluctl.io/request-deploy": '{"requestValue":"new"}' },
      },
      status: {
        ...deployment.status,
        deployRequestResult: {
          request: { requestValue: "old" },
          endTime: "2025-02-01T10:00:00Z",
        },
      },
    }
    expect(requestState(updated, "deploy").state).toEqual("Requested")
    updated.status.deployRequestResult.request.requestValue = "new"
    expect(requestState(updated, "deploy").state).toEqual("Completed")
    const [row] = targetsFrom(
      applyEvents(emptyInventory(), [{ ...crEvent, deployment: updated }])
    )
    expect(reconciliation(row).label).toEqual("Unknown")
  })
})

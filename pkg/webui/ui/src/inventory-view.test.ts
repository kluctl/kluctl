import { describe, expect, it } from "vitest"
import { type Deployment, type TargetRow } from "./domain"
import {
  matchesFilter,
  matchesSearch,
  projectGroups,
  targetHealth,
  targetIndicators,
} from "./inventory-view"
import type {
  DriftDetectionResult,
  ValidateResult,
  ValidateResultSummary,
} from "./models"

const deployment: Deployment = {
  metadata: { uid: "one", name: "shop-prod", namespace: "apps" },
  spec: { validate: true },
  status: { conditions: [{ type: "Ready", status: "True" }] },
}
const row: TargetRow = {
  id: "one",
  project: {
    repoKey: "git://example.com/team/catalog",
    subDir: "environments/store",
  },
  target: {
    targetName: "production",
    clusterId: "workload",
    discriminator: "store-prod",
  },
  instance: { clusterId: "management", namespace: "apps", name: "shop-prod" },
  deployment,
  runs: [],
}

const drift = {
  renderedObjectsHash: "latest",
  objects: [
    {
      ref: { kind: "Deployment", name: "store" },
      changes: [{ jsonPath: "spec.replicas" }],
    },
  ],
} as DriftDetectionResult

function withStatus(
  status: Deployment["status"],
  spec = deployment.spec
): TargetRow {
  return {
    ...row,
    deployment: {
      ...deployment,
      spec,
      status: { ...deployment.status, ...status },
    },
  }
}

describe("project inventory", () => {
  it("groups deployments by repo and directory and keeps records separate", () => {
    const historical = { ...row, id: "older", deployment: undefined }
    const cli = {
      ...row,
      id: "cli",
      deployment: undefined,
      instance: undefined,
    }
    const second = {
      ...row,
      id: "two",
      instance: { ...row.instance!, name: "shop-staging" },
    }
    const otherDir = {
      ...row,
      id: "subdir",
      project: { ...row.project, subDir: "store" },
    }
    const otherRepo = {
      ...row,
      id: "repo",
      project: { ...row.project, repoKey: "git://example.com/other/catalog" },
    }
    const groups = projectGroups([
      otherRepo,
      row,
      cli,
      second,
      historical,
      otherDir,
    ])
    expect(groups).toHaveLength(3)
    const main = groups.find((project) => project.deployments.length === 2)!
    expect(main.name).toBe("store")
    expect(main.deployments.map(({ id }) => id)).toEqual(["one", "two"])
    expect(main.recorded.map(({ id }) => id)).toEqual(["cli", "older"])
  })

  it("searches across project, deployment, target and cluster identity", () => {
    expect(matchesSearch(row, "  CATALOG production\tapps Workload ")).toBe(
      true
    )
    expect(matchesSearch(row, "shop-prod management")).toBe(true)
    expect(matchesSearch(row, "shop-prod staging")).toBe(false)
  })
})

describe("deployment indicators", () => {
  it("keeps drift, warnings, and errors independent even when reconciliation succeeded", () => {
    const target = withStatus({
      lastDriftDetectionResult: drift,
      lastValidateResult: {
        ready: true,
        warnings: [{ message: "Warning" }],
      } as ValidateResult,
    })
    expect(targetIndicators(target)).toEqual(["warning", "drift"])
    expect(matchesFilter(targetHealth(target), "warnings")).toBe(true)
    expect(matchesFilter(targetHealth(target), "drift")).toBe(true)
    expect(matchesFilter(targetHealth(target), "errors")).toBe(false)

    target.validation = { ready: false, errors: 0 } as ValidateResultSummary
    expect(targetIndicators(target)).toEqual(["error", "drift"])
    expect(matchesFilter(targetHealth(target), "important")).toBe(true)
  })

  it("does not report a suspended or unapproved first rollout as a failure", () => {
    const status = {
      conditions: [
        {
          type: "Ready",
          status: "False",
          reason: "DeployFailed",
          message: "deployment status unknown",
        },
      ],
    }
    expect(targetIndicators(withStatus(status, { suspend: true }))).toEqual([
      "suspended",
    ])
    const manual = withStatus(
      { ...status, lastDriftDetectionResult: drift },
      { manual: true }
    )
    expect(targetIndicators(manual)).toEqual(["drift", "approval"])
    expect(matchesFilter(targetHealth(manual), "errors")).toBe(false)
    manual.deployment!.spec.manualObjectsHash = "latest"
    expect(targetIndicators(manual)).toEqual(["drift"])
  })

  it("does not present pending deployments, dry runs, or recorded CLI targets as live and ready", () => {
    expect(targetIndicators(row)).toEqual(["ready"])
    expect(targetIndicators(withStatus({ conditions: [] }))).toEqual([
      "pending",
    ])
    expect(targetIndicators(withStatus({}, { dryRun: true }))).toEqual([
      "dryRun",
    ])
    expect(
      targetIndicators({ ...row, deployment: undefined, instance: undefined })
    ).toEqual(["recorded"])
  })
})

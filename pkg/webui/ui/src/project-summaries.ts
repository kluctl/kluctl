import { CommandResultSummary, KluctlDeploymentInfo, ProjectKey, TargetKey, ValidateResultSummary, } from "./models";
import _ from "lodash";
import { KluctlDeploymentWithClusterId } from "./components/App";
import { ActiveFilters, DoFilterSwitches, DoFilterText } from "./components/FilterBar";

export interface TargetSummary {
    target: TargetKey;
    kdInfo?: KluctlDeploymentInfo
    kd?: KluctlDeploymentWithClusterId;
    lastValidateResult?: ValidateResultSummary;
    commandResults: CommandResultSummary[];
}

export interface ProjectSummary {
    project: ProjectKey;
    targets: TargetSummary[];
}

export function compareCommandResultSummaries(a: CommandResultSummary, b: CommandResultSummary) {
    return b.commandInfo.startTime.localeCompare(a.commandInfo.startTime) ||
        b.commandInfo.endTime.localeCompare(b.commandInfo.endTime) ||
        (b.commandInfo.command || "").localeCompare(a.commandInfo.command || "")
}

export function compareValidateResultSummaries(a: ValidateResultSummary, b: ValidateResultSummary) {
    return b.startTime.localeCompare(a.startTime) ||
        b.endTime.localeCompare(b.endTime)
}

export function buildProjectSummaries(commandResultSummaries: Map<string, CommandResultSummary>,
                                      validateResultSummaries: Map<string, ValidateResultSummary>,
                                      kluctlDeployments: Map<string, KluctlDeploymentWithClusterId>,
                                      filters?: ActiveFilters) {
    const filterTarget = (kd1: KluctlDeploymentWithClusterId | undefined, kd2: KluctlDeploymentInfo | undefined, projectKey: ProjectKey, targetKey: TargetKey) => {
        if (kd1 && DoFilterText([
            kd1.clusterId,
            kd1.deployment.metadata.name,
            kd1.deployment.metadata.namespace,
        ], filters)) {
            return true
        }
       if (kd2 && DoFilterText([
           kd2.clusterId,
           kd2.name,
           kd2.namespace,
       ], filters)) {
           return true
       }
        if (DoFilterText([
            projectKey.gitRepoKey,
            projectKey.subDir,
            targetKey.targetName,
            targetKey.clusterId,
            targetKey.discriminator
        ], filters)) {
            return true
        }
        return false
    }

    const filterTargetByStatus = (ts: TargetSummary) => {
        let hasErrors = !!ts.lastValidateResult?.errors
        let hasWarnings = !!ts.lastValidateResult?.warnings
        let hasChanges = false

        const conditions: any[] | undefined = ts.kd?.deployment.status?.conditions
        const readyCondition = conditions?.find(c => c.type === "Ready")
        if (readyCondition && readyCondition.status === "False") {
            hasErrors = true
        }

        if (ts.commandResults.length) {
            hasErrors = hasErrors || !!ts.commandResults[0].errors?.length
            hasWarnings = hasWarnings || !!ts.commandResults[0].warnings?.length
            hasChanges = hasChanges || !!ts.commandResults[0].changedObjects
        }
        return DoFilterSwitches(hasChanges, hasErrors, hasWarnings, filters)
    }

    const sortedCommandResults = Array.from(commandResultSummaries.values())
    sortedCommandResults.sort(compareCommandResultSummaries)

    const buildKdKey = (clusterId: string, name: string, namespace: string) => {
        return clusterId + "-" + name + "-" + namespace
    }

    const kdByNameAndClusterId = new Map<string, KluctlDeploymentWithClusterId>()
    kluctlDeployments.forEach(kd => {
        kdByNameAndClusterId.set(buildKdKey(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace), kd)
    })

    const m = new Map<string, ProjectSummary>()

    const getOrCreateProject = (project: ProjectKey, allowCreate: boolean) => {
        const projectKey = JSON.stringify(project)
        let p = m.get(projectKey)
        if (!p && allowCreate) {
            p = {
                project: project,
                targets: []
            }
            m.set(projectKey, p)
        }
        return p
    }
    const getOrCreateTarget = (project: ProjectKey, targetKey: TargetKey, kdInfo: KluctlDeploymentInfo | undefined, allowCreateProject: boolean, allowCreateTarget: boolean) => {
        const p = getOrCreateProject(project, allowCreateProject)
        if (!p) {
            return undefined
        }
        let t = p.targets.find(t => _.isEqual(t.target, targetKey) && _.isEqual(t.kdInfo, kdInfo))
        if (!t && allowCreateTarget) {
            t = {
                target: targetKey,
                kdInfo: kdInfo,
                commandResults: []
            }
            p.targets.push(t)
        }
        return t
    }

    const kluctlDeploymentsByKdKey = new Map<string, KluctlDeploymentWithClusterId>()
    kluctlDeployments.forEach(kd => {
        if (!kd.deployment.status || !kd.deployment.status.projectKey) {
            return
        }
        if (!filterTarget(kd, undefined, kd.deployment.status.projectKey, kd.deployment.status.targetKey)) {
            return
        }

        const kdInfo = {
            "clusterId": kd.clusterId,
            "name": kd.deployment.metadata.name,
            "namespace": kd.deployment.metadata.namespace,
        }
        kluctlDeploymentsByKdKey.set(buildKdKey(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace), kd)

        const target = getOrCreateTarget(kd.deployment.status.projectKey, kd.deployment.status.targetKey, kdInfo,true, true)
        target!.kd = kd
    })

    sortedCommandResults.forEach(rs => {
        if (rs.commandInfo.kluctlDeployment) {
            // filter out command results from KluctlDeployments for which the KluctlDeployment itself vanished
            const key = buildKdKey(rs.commandInfo.kluctlDeployment.clusterId, rs.commandInfo.kluctlDeployment.name, rs.commandInfo.kluctlDeployment.namespace)
            if (!kluctlDeploymentsByKdKey.has(key)) {
                return
            }
        }

        if (!filterTarget(undefined, rs.commandInfo.kluctlDeployment, rs.projectKey, rs.targetKey)) {
            return
        }

        const target = getOrCreateTarget(rs.projectKey, rs.targetKey, rs.commandInfo.kluctlDeployment, true, true)
        if (!target) {
            return
        }

        target.commandResults.push(rs)
    })

    validateResultSummaries.forEach(vr => {
        const target = getOrCreateTarget(vr.projectKey, vr.targetKey, vr.kluctlDeployment, false, false)
        if (!target) {
            return
        }

        if (!target.lastValidateResult || vr.startTime > target.lastValidateResult.startTime) {
            target.lastValidateResult = vr
        }
    })

    m.forEach((ps, key) => {
        ps.targets = ps.targets.filter(ts => {
            if (!filterTargetByStatus(ts)) {
                return false
            }
            return true
        })
        if (!ps.targets.length) {
            m.delete(key)
        }
    })

    const ret: ProjectSummary[] = []
    m.forEach(p => {
        p.targets.sort((a, b) => {
            return (a.target.targetName || "").localeCompare(b.target.targetName || "") ||
                a.target.clusterId.localeCompare(b.target.clusterId) ||
                (a.target.discriminator || "")?.localeCompare(b.target.discriminator || "")
        })
        ret.push(p)
    })

    ret.sort((a, b) => {
        return (a.project.gitRepoKey || "").localeCompare(b.project.gitRepoKey || "") ||
            (a.project.subDir || "").localeCompare(b.project.subDir || "")
    })

    return ret
}
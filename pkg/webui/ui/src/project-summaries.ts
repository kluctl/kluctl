import {
    CommandResultSummary,
    KluctlDeploymentInfo,
    ProjectKey,
    ProjectTargetKey,
    TargetKey,
    ValidateResultSummary,
} from "./models";
import _ from "lodash";
import { KluctlDeploymentWithClusterId } from "./components/App";

export interface TargetSummary {
    target: TargetKey;
    kluctlDeployments: Map<string, KluctlDeploymentWithClusterId>;
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
                                      includeWithoutKluctlDeployments: boolean) {
    const filter = (projectKey: ProjectKey, targetKey: TargetKey) => {
        return true
    }


    const sortedCommandResults = Array.from(commandResultSummaries.values())
    sortedCommandResults.sort(compareCommandResultSummaries)

    const vrByProjectTargetKey = new Map<string, ValidateResultSummary[]>()
    validateResultSummaries.forEach(vr=> {
        if (!filter(vr.projectKey, vr.targetKey)) return

        console.log(vr)

        const key = new ProjectTargetKey()
        key.project = vr.projectKey
        key.target = vr.targetKey

        const keyStr = JSON.stringify(key)
        let l = vrByProjectTargetKey.get(keyStr)
        if (!l) {
            l = []
            vrByProjectTargetKey.set(keyStr, l)
        }
        l.push(vr)
    })
    vrByProjectTargetKey.forEach(l => {
        l.sort(compareValidateResultSummaries)
    })

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
    const getOrCreateTarget = (project: ProjectKey, targetKey: TargetKey, allowCreateProject: boolean, allowCreateTarget: boolean) => {
        const p = getOrCreateProject(project, allowCreateProject)
        if (!p) {
            return undefined
        }
        let t = p.targets.find(t => _.isEqual(t.target, targetKey))
        if (!t && allowCreateTarget) {
            t = {
                target: targetKey,
                kluctlDeployments: new Map<string, KluctlDeploymentWithClusterId>(),
                commandResults: []
            }
            p.targets.push(t)
        }
        return t
    }

    kluctlDeployments.forEach(kd => {
        if (!kd.deployment.status || !kd.deployment.status.projectKey) {
            return
        }
        if (!filter(kd.deployment.status.projectKey, kd.deployment.status.targetKey)) return

        console.log("kd", kd)

        const target = getOrCreateTarget(kd.deployment.status.projectKey, kd.deployment.status.targetKey, true, true)
        target!.kluctlDeployments.set(buildKdKey(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace), kd)
    })

    sortedCommandResults.forEach(rs => {
        if (!filter(rs.projectKey, rs.targetKey)) return

        console.log(rs)

        const target = getOrCreateTarget(rs.projectKey, rs.targetKey, includeWithoutKluctlDeployments, includeWithoutKluctlDeployments)
        if (!target) {
            return
        }

        target.commandResults.push(rs)
    })

    validateResultSummaries.forEach(vr => {
        if (!filter(vr.projectKey, vr.targetKey)) return

        const target = getOrCreateTarget(vr.projectKey, vr.targetKey, false, false)
        if (!target) {
            return
        }

        if (!target.lastValidateResult) {
            target.lastValidateResult = vr
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
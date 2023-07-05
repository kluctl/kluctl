import { CommandResultSummary, ProjectKey, ProjectTargetKey, TargetKey, ValidateResultSummary, } from "./models";
import _ from "lodash";

export interface TargetSummary {
    target: TargetKey;
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

export function buildProjectSummaries(commandResultSummaries: Map<string, CommandResultSummary>, validateResultSummaries: Map<string, ValidateResultSummary>) {
    const sortedCommandResults = Array.from(commandResultSummaries.values())
    sortedCommandResults.sort(compareCommandResultSummaries)

    const vrByProjectTargetKey = new Map<string, ValidateResultSummary[]>()
    validateResultSummaries.forEach(vr=> {
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

    const m = new Map<string, ProjectSummary>()
    sortedCommandResults.forEach(rs => {
        const projectKey = JSON.stringify(rs.projectKey)

        let p = m.get(projectKey)
        if (!p) {
            p = {
                project: rs.projectKey,
                targets: []
            }
            m.set(projectKey, p)
        }

        const ptKey = new ProjectTargetKey()
        ptKey.project = rs.projectKey
        ptKey.target = rs.targetKey

        const vrs = vrByProjectTargetKey.get(JSON.stringify(ptKey))
        const vr = vrs ? vrs[0] : undefined

        let target = p.targets.find(t => _.isEqual(t.target, rs.targetKey))
        if (!target) {
            target = {
                target: rs.targetKey,
                lastValidateResult: vr,
                commandResults: []
            }
            p.targets.push(target)
        }

        target.commandResults.push(rs)
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
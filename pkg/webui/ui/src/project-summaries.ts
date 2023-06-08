import {
    CommandResultSummary,
    ProjectKey,
    ProjectTargetKey, TargetKey, ValidateResult,
} from "./models";
import _ from "lodash";

export interface TargetSummary {
    target: TargetKey;
    lastValidateResult?: ValidateResult;
    commandResults: CommandResultSummary[];
}

export interface ProjectSummary {
    project: ProjectKey;
    targets: TargetSummary[];
}

export function compareSummaries(a: CommandResultSummary, b: CommandResultSummary) {
    return b.commandInfo.startTime.localeCompare(a.commandInfo.startTime) ||
        b.commandInfo.endTime.localeCompare(b.commandInfo.endTime) ||
        (b.commandInfo.command || "").localeCompare(a.commandInfo.command || "")
}

export function buildProjectSummaries(summaries: Map<string, CommandResultSummary>, validateResults: Map<string, ValidateResult>) {
    const sorted = Array.from(summaries.values())
    sorted.sort(compareSummaries)

    const m = new Map<string, ProjectSummary>()
    sorted.forEach(rs => {
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

        const vr = validateResults.get(JSON.stringify(ptKey))

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
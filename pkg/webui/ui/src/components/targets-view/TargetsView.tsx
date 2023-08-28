import { CommandResultSummary } from "../../models";
import React, { useCallback, useMemo } from "react";
import { useAppContext } from "../App";
import { buildTargetKey, ProjectSummary, TargetSummary } from "../../project-summaries";
import { useLocation, useNavigate } from "react-router-dom";
import { TargetCardsView } from "../target-cards-view/TargetCardsView";

export const TargetsView = () => {
    const navigate = useNavigate();
    const loc = useLocation();
    const appContext = useAppContext();
    const projects = appContext.projects;

    const onSelect = useCallback((ps: ProjectSummary, ts: TargetSummary, showResults: boolean, rs?: CommandResultSummary | undefined) => {
        let p = `/targets/${buildTargetKey(ps.project, ts.target, ts.kdInfo)}`
        if (showResults) {
            p += "/results"
            if (rs) {
                p += "/" + rs.id
            }
        }
        navigate(p);
    }, [navigate]);

    const onCloseExpanded = useCallback(() => {
        navigate(`/targets/`);
    }, [navigate]);

    const targetsByKey = useMemo(() => {
        const m = new Map<string, {ps: ProjectSummary, ts: TargetSummary}>()
        projects.forEach(ps => {
            ps.targets.forEach(ts => {
                const key = buildTargetKey(ps.project, ts.target, ts.kdInfo)
                m.set(key, {ps: ps, ts: ts})
            })
        })
        return m
    }, [projects])

    const pathnameS = loc.pathname.split("/")
    const selectedTargetKey = pathnameS[2]
    const selected = targetsByKey.get(selectedTargetKey)

    let selectedCommandResult: CommandResultSummary | undefined
    if (selected) {
        if (pathnameS[3] === "results") {
            const resultId = pathnameS[4]
            if (resultId) {
                selectedCommandResult = appContext.commandResultSummaries.get(resultId)
            }
        }
    }

    return <TargetCardsView
        selectedProject={selected?.ps}
        selectedTarget={selected?.ts}
        selectedResult={selectedCommandResult}
        onSelect={onSelect}
        onCloseExpanded={onCloseExpanded}
    />
}

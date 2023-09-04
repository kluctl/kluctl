import { CommandResultSummary } from "../../models";
import React, { useCallback, useMemo } from "react";
import { useAppContext } from "../App";
import { buildTargetKey, ProjectSummary, TargetSummary } from "../../project-summaries";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { TargetCardsView } from "../target-cards-view/TargetCardsView";
import { TargetsListView } from "../target-list-view/TargetsListView";

export const TargetsView = () => {
    const navigate = useNavigate();
    const loc = useLocation();
    const appContext = useAppContext();
    const projects = appContext.projects;
    const [searchParams] = useSearchParams()

    const fullResult = searchParams.get("full") === "1"

    const doNavigate = useCallback((p: string, sp?: URLSearchParams) => {
        sp = new URLSearchParams(sp)
        const qs = sp.toString()
        if (qs.length) {
            p += "?" + qs
        }
        navigate(p)
    }, [navigate])

    const onSelect = useCallback((ps: ProjectSummary, ts: TargetSummary, showResults: boolean, rs?: CommandResultSummary, full?: boolean) => {
        let p = `/targets/${buildTargetKey(ps.project, ts.target, ts.kdInfo)}`
        const sp = new URLSearchParams()
        if (showResults) {
            p += "/results"
            if (rs) {
                p += "/" + rs.id
                if (full) {
                    console.log("full")
                    sp.set("full", "1")
                }
            }
        }
        doNavigate(p, sp);
    }, [doNavigate]);

    const onCloseExpanded = useCallback(() => {
        doNavigate(`/targets/`);
    }, [doNavigate]);

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

    if (!appContext.filters?.showTableView) {
        return <TargetCardsView
            selectedProject={selected?.ps}
            selectedTarget={selected?.ts}
            selectedResult={selectedCommandResult}
            selectedResultFull={fullResult}
            onSelect={onSelect}
            onCloseExpanded={onCloseExpanded}
        />
    } else {
        return <TargetsListView selectedProject={selected?.ps}
                                selectedTarget={selected?.ts}
                                selectedResult={selectedCommandResult}
                                selectedResultFull={fullResult}
                                onSelect={onSelect}
                                onCloseExpanded={onCloseExpanded}
        />
    }
}

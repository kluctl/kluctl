import { CommandResultSummary, KluctlDeploymentInfo, ProjectKey, TargetKey } from "../../models";
import { Box, Typography } from "@mui/material";
import React, { useCallback, useMemo } from "react";
import { useAppContext } from "../App";
import { ProjectCard } from "./ProjectCard";
import { TargetCard } from "./TargetCard";
import Divider from "@mui/material/Divider";
import { CardCol, cardGap, cardHeight, CardPaper, CardRow, cardWidth } from "../card/Card";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { buildListKey } from "../../utils/listKey";
import { ExpandableCard } from "../card/ExpandableCard";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { CommandResultCard } from "../command-result/CommandResultCard";
import { RelationHorizontalLine, RelationTree } from "./Relations";

const colWidth = 416;

function ColHeader({ children }: { children: React.ReactNode }) {
    return <Box
        minWidth={colWidth}
        width={colWidth}
        height='42px'
        display='flex'
        alignItems='center'
        sx={{
            borderLeft: '0.8px solid rgba(0,0,0,0.5)',
            paddingLeft: '15px',
            '&:first-of-type': {
                borderLeft: 'none',
                paddingLeft: 0
            }
        }}
    >
        <Typography variant='h2' textAlign='left'>{children}</Typography>
    </Box>
}

function buildTargetKey(project: ProjectKey, target: TargetKey, kluctlDeployment?: KluctlDeploymentInfo) {
    const j = {
        "project": project,
        "target": target,
        "kluctlDeployment": kluctlDeployment,
    }
    return buildListKey(j)
}

export const TargetCardsView = () => {
    const navigate = useNavigate();
    const loc = useLocation();
    const [searchParams] = useSearchParams()
    const appContext = useAppContext();
    const projects = appContext.projects;

    const fullResultStr = searchParams.get("full")
    const fullResult = fullResultStr === "" || fullResultStr === "1"

    const onSelectTargetItem = useCallback((ps: ProjectSummary, ts: TargetSummary) => {
        navigate(`/targets/${buildTargetKey(ps.project, ts.target, ts.kdInfo)}`);
    }, [navigate]);

    const onSelectCommandResultItem = useCallback((ps: ProjectSummary, ts: TargetSummary, rs: CommandResultSummary | undefined, forceFull: boolean | undefined) => {
        let p = `/targets/${buildTargetKey(ps.project, ts.target, ts.kdInfo)}/results`
        if (rs) {
            p += "/" + rs.id
            let full = false
            if (forceFull !== undefined) {
                full = forceFull
            } else {
                full = fullResult
            }
            if (full) {
                p += "?full=1"
            }
        }
        navigate(p);
    }, [navigate, fullResult]);

    const onCardClose = useCallback(() => {
        navigate(`/targets/`);
    }, [navigate]);

    const targetsByKey = useMemo(() => {
        const m = new Map<string, TargetSummary>()
        projects.forEach(ps => {
            ps.targets.forEach(ts => {
                const key = buildTargetKey(ps.project, ts.target, ts.kdInfo)
                m.set(key, ts)
            })
        })
        return m
    }, [projects])

    const pathnameS = loc.pathname.split("/")
    let selectedTargetKey = pathnameS[2]
    const selectedTarget = targetsByKey.get(selectedTargetKey)

    if (!selectedTarget) {
        selectedTargetKey = ""
    }

    let expandedResults = false
    let expandedResultId = ""
    if (selectedTarget) {
        if (pathnameS[3] === "results") {
            expandedResults = true
            const resultId = pathnameS[4]
            if (resultId) {
                if (appContext.commandResultSummaries.has(resultId)) {
                    expandedResultId = resultId
                }
            }
        }
    }


    return <Box minWidth={colWidth * 3} height={"100%"} p='0 40px'>
        <Box display={"flex"} alignItems={"center"} height='70px'>
            <ColHeader>Projects</ColHeader>
            <ColHeader>Targets</ColHeader>
            <ColHeader>Command Results</ColHeader>
        </Box>
        <Divider />
        {projects.map((ps, i) => {
            return <Box key={buildListKey(ps.project)}>
                <Box display={"flex"} alignItems={"center"} margin='40px 0'>
                    <Box display='flex' alignItems='center' width={colWidth} flex='0 0 auto'>
                        <ProjectCard ps={ps} />
                        <Box
                            flexGrow={1}
                            height={ps.targets.length * cardHeight + (ps.targets.length - 1) * cardGap}
                            display='flex'
                            justifyContent='center'
                            alignItems='center'
                        >
                            <RelationTree targetCount={ps.targets.length} />
                        </Box>
                    </Box>

                    <CardCol width={colWidth} flex='0 0 auto'>
                        {ps.targets.map((ts, i) => {
                            const key = buildTargetKey(ps.project, ts.target, ts.kdInfo)
                            return <Box key={key} display='flex'>
                                <ExpandableCard
                                    cardWidth={cardWidth}
                                    cardHeight={cardHeight}
                                    expand={!expandedResults && selectedTargetKey === key}
                                    onExpand={() => onSelectTargetItem(ps, ts)}
                                    onClose={() => {
                                        onCardClose()
                                    }}
                                    onSelect={cd => {
                                    }}
                                    cardsData={[ts]}
                                    getKey={cd => buildListKey([cd.target, cd.kdInfo])}
                                    selected={selectedTargetKey}
                                    renderCard={(cardData, expanded, current) => {
                                        return <TargetCard
                                            ps={ps}
                                            ts={ts}
                                            expanded={expanded}
                                            onClose={() => {
                                                onCardClose()
                                            }}
                                        />
                                    }}/>
                                {ts.commandResults.length ? <RelationHorizontalLine/> : <></>}
                            </Box>
                        })}
                    </CardCol>

                    <CardCol width={colWidth}>
                        {ps.targets.map((ts, i) => {
                            const tsKey = buildTargetKey(ps.project, ts.target, ts.kdInfo)
                            return <CardRow key={tsKey} height={cardHeight}>
                                {ts.commandResults?.slice(0, 4).map((rs, i) => {
                                    return i === 0 ? <ExpandableCard
                                            key={rs.id}
                                            cardWidth={cardWidth}
                                            cardHeight={cardHeight}
                                            expand={expandedResults && selectedTargetKey === tsKey}
                                            onExpand={() => {
                                                onSelectCommandResultItem(ps, ts, rs, false)
                                            }}
                                            onClose={() => {
                                                onCardClose()
                                            }}
                                            onSelect={cd => {
                                                onSelectCommandResultItem(ps, ts, cd, fullResult)
                                            }}
                                            selected={expandedResultId}
                                            cardsData={ts.commandResults}
                                            getKey={cd => cd.id}
                                            renderCard={(cardData, expanded, current) => {
                                                return <CommandResultCard
                                                    current={current}
                                                    ps={ps}
                                                    ts={ts}
                                                    rs={cardData}
                                                    showSummary={!fullResult}
                                                    expanded={expanded}
                                                    loadData={expanded && current}
                                                    onClose={() => {
                                                        onCardClose()
                                                    }}
                                                    onSwitchFullCommandResult={() => {
                                                        onSelectCommandResultItem(ps, ts, cardData, !fullResult)
                                                    }}
                                                />
                                            }}/>
                                        : <CardPaper
                                            key={rs.id}
                                            sx={{
                                                width: cardWidth,
                                                height: cardHeight,
                                                translate: `${-i * (cardWidth + cardGap / 2)}px`,
                                                zIndex: -i,
                                            }}
                                        />
                                })}
                            </CardRow>
                        })}
                    </CardCol>
                </Box>
                <Divider />
            </Box>
        })}
    </Box>
}

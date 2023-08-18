import { CommandResultSummary, KluctlDeploymentInfo, ProjectKey, TargetKey } from "../../models";
import { Box, Typography, useTheme } from "@mui/material";
import React, { useCallback, useContext, useMemo } from "react";
import { AppContext } from "../App";
import { ProjectItem } from "./ProjectItem";
import { TargetItem } from "./TargetItem";
import Divider from "@mui/material/Divider";
import { CardCol, cardGap, cardHeight, CardPaper, CardRow, cardWidth } from "./Card";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { buildListKey } from "../../utils/listKey";
import { ExpandableCard } from "./ExpandableCard";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { CommandResultCard } from "../command-result/CommandResultCard";

const colWidth = 416;
const curveRadius = 12;
const circleRadius = 5;
const strokeWidth = 2;

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

const Circle = React.memo((props: React.SVGProps<SVGCircleElement>) => {
    const theme = useTheme();
    return <circle
        r={circleRadius}
        fill={theme.palette.background.default}
        stroke={theme.palette.secondary.main}
        strokeWidth={strokeWidth}
        {...props}
    />
})

const RelationHorizontalLine = React.memo(() => {
    const theme = useTheme();
    return <Box flexGrow={1} display='flex' justifyContent='center' alignItems='center' px='9px'>
        <svg
            xmlns='http://www.w3.org/2000/svg'
            fill='none'
            height={`${2 * circleRadius + strokeWidth}px`}
            width='100%'
        >
            <svg
                height='100%'
                width='100%'
                viewBox={`0 0 100 ${2 * circleRadius + strokeWidth}`}
                fill='none'
                preserveAspectRatio='none'
            >
                <path
                    d={`
                  M ${circleRadius + strokeWidth / 2} ${circleRadius + strokeWidth / 2}
                  H ${100 - circleRadius - strokeWidth / 2}
                `}
                    stroke={theme.palette.secondary.main}
                    strokeWidth={strokeWidth}
                />
            </svg>
            <svg
                fill='none'
                height='100%'
                width='100%'
            >
                <Circle cx={circleRadius + strokeWidth / 2} cy='50%' />
            </svg>
            <svg
                fill='none'
                height='100%'
                width='100%'
            >
                <Circle cx={`calc(100% - ${circleRadius + strokeWidth / 2}px)`} cy='50%' />
            </svg>
        </svg>
    </Box>;
});

const RelationTree = React.memo(({ targetCount }: { targetCount: number }): JSX.Element | null => {
    const theme = useTheme();
    const height = targetCount * cardHeight + (targetCount - 1) * cardGap
    const width = 152;

    if (targetCount <= 0) {
        return null;
    }

    const targetsCenterYs = Array(targetCount).fill(0).map((_, i) =>
        cardHeight / 2 + i * (cardHeight + cardGap)
    );
    const rootCenterY = height / 2;

    return <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        fill='none'
    >
        {targetsCenterYs.map((cy, i) => {
            let d: React.SVGAttributes<SVGPathElement>['d'];
            if (targetCount % 2 === 1 && i === Math.floor(targetCount / 2)) {
                // target is in the middle.
                d = `
                        M ${circleRadius},${rootCenterY}
                        h ${width - circleRadius}
                    `;
            } else if (i < targetCount / 2) {
                // target is higher than root.
                d = `
                        M ${circleRadius},${rootCenterY}
                        h ${width / 2 - curveRadius - circleRadius}
                        a ${curveRadius} ${curveRadius} 90 0 0 ${curveRadius} -${curveRadius}
                        v ${cy - rootCenterY + curveRadius * 2}
                        a ${curveRadius} ${curveRadius} 90 0 1 ${curveRadius} -${curveRadius}
                        h ${width / 2 - curveRadius - circleRadius}
                    `;
            } else {
                // target is lower than root.
                d = `
                    M ${circleRadius},${rootCenterY}
                    h ${width / 2 - curveRadius - circleRadius}
                    a ${curveRadius} ${curveRadius} 90 0 1 ${curveRadius} ${curveRadius}
                    v ${cy - rootCenterY - curveRadius * 2}
                    a ${curveRadius} ${curveRadius} 90 0 0 ${curveRadius} ${curveRadius}
                    h ${width / 2 - curveRadius - circleRadius}
                `;
            }

            return [
                <path
                    key={`path-${i}`}
                    d={d}
                    stroke={theme.palette.secondary.main}
                    strokeWidth={strokeWidth}
                    strokeLinecap='round'
                    strokeLinejoin='round'
                />,
                <Circle
                    key={`circle-${i}`}
                    cx={width - circleRadius - strokeWidth / 2}
                    cy={cy}
                />
            ]
        })}
        <Circle
            key='circle-root'
            cx={circleRadius + strokeWidth / 2}
            cy={rootCenterY}
        />
    </svg>
});

function buildTargetKey(project: ProjectKey, target: TargetKey, kluctlDeployment?: KluctlDeploymentInfo) {
    const j = {
        "project": project,
        "target": target,
        "kluctlDeployment": kluctlDeployment,
    }
    return buildListKey(j)
}

export const TargetsView = () => {
    const navigate = useNavigate();
    const loc = useLocation();
    const [searchParams] = useSearchParams()
    const appContext = useContext(AppContext);
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
                        <ProjectItem ps={ps} />
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
                                        return <TargetItem
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
                                                onSelectCommandResultItem(ps, ts, cd, false)
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
                                                    loadData={current}
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

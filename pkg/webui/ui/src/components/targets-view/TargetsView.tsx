import { CommandResultSummary, KluctlDeploymentInfo, ProjectKey, TargetKey } from "../../models";
import { Box, Typography, useTheme } from "@mui/material";
import React, { useCallback, useContext, useMemo, useRef } from "react";
import { AppContext } from "../App";
import { ProjectItem } from "./ProjectItem";
import { TargetItem } from "./TargetItem";
import Divider from "@mui/material/Divider";
import { CommandResultItem } from "./CommandResultItem";
import { CardCol, cardGap, cardHeight, CardPaper, CardRow, cardWidth } from "./Card";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { buildListKey } from "../../utils/listKey";
import { ExpandedCardsView } from "./ExpandedCardsView";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import _ from "lodash";

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

interface TargetPath {
    project: ProjectKey
    target: TargetKey
    kluctlDeployment?: KluctlDeploymentInfo
}

function buildTargetPath(project: ProjectKey, target: TargetKey, kluctlDeployment?: KluctlDeploymentInfo): string {
    return btoa(JSON.stringify({
        "project": project,
        "target": target,
        "kluctlDeployment": kluctlDeployment,
    }))
}

function parseTargetPath(s?: string): TargetPath | undefined {
    if (!s) {
        return
    }
    return JSON.parse(atob(s))
}

export const TargetsView = () => {
    const navigate = useNavigate();
    const { targetPath} = useParams();
    const { pathname } = useLocation();
    const appContext = useContext(AppContext);
    const projects = appContext.projects;

    const onSelectTargetItem = useCallback((ps: ProjectSummary, ts: TargetSummary) => {
        navigate(`/targets/${buildTargetPath(ps.project, ts.target, ts.kdInfo)}`);
    }, [navigate]);

    const onSelectCommandResultItem = useCallback((ps: ProjectSummary, ts: TargetSummary) => {
        navigate(`/targets/${buildTargetPath(ps.project, ts.target, ts.kdInfo)}/history`);
    }, [navigate]);

    const onCardClose = useCallback(() => {
        navigate(`/targets/`);
    }, [navigate]);

    const commandResultItemRefs = useRef<Map<TargetSummary, HTMLElement>>(new Map());
    const targetItemRefs = useRef<Map<TargetSummary, HTMLElement>>(new Map());

    const [parentProject, selectedTarget] = useMemo(() => {
        const tp = parseTargetPath(targetPath)
        if (!tp) {
            return [undefined, undefined]
        }

        const project = projects.find(ps => _.isEqual(_.toPlainObject(ps.project), tp.project))
        const target = project?.targets.find(ts => _.isEqual(_.toPlainObject(ts.target), tp.target) && (ts.kdInfo === tp.kluctlDeployment || _.isEqual(_.toPlainObject(ts.kdInfo), tp.kluctlDeployment)))
        return [project, target]
    }, [projects, targetPath])

    const showHistory = pathname.endsWith('history');

    if (selectedTarget && parentProject && !showHistory) {
        const cardElem = targetItemRefs.current.get(selectedTarget);
        const cardRect = cardElem?.getBoundingClientRect();

        return <ExpandedCardsView<TargetSummary>
            cardsData={[selectedTarget]}
            renderCard={(cardData, sx, expanded) =>
                <TargetItem
                    ps={parentProject}
                    ts={cardData}
                    sx={sx}
                    key={targetPath}
                    expanded={expanded}
                    onClose={onCardClose}
                />
            }
            initialCardRect={cardRect}
            onClose={onCardClose}
        />;
    }

    if (selectedTarget && showHistory) {
        const cardElem = commandResultItemRefs.current.get(selectedTarget);
        const cardRect = cardElem?.getBoundingClientRect();

        return <ExpandedCardsView<CommandResultSummary>
            cardsData={selectedTarget.commandResults}
            renderCard={(cardData, sx, expanded, current) =>
                <CommandResultItem
                    rs={cardData}
                    sx={sx}
                    key={cardData.id}
                    expanded={expanded}
                    loadData={current}
                    onClose={onCardClose}
                />
            }
            initialCardRect={cardRect}
            onClose={onCardClose}
        />;
    }

    return <Box minWidth={colWidth * 3} p='0 40px'>
        <Box display={"flex"} alignItems={"center"} height='70px'>
            <ColHeader>Projects</ColHeader>
            <ColHeader>Targets</ColHeader>
            <ColHeader>History</ColHeader>
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
                            return <Box key={buildListKey([ts.target, ts.kdInfo])} display='flex'>
                                <TargetItem
                                    ps={ps}
                                    ts={ts}
                                    onSelectTarget={() => onSelectTargetItem(ps, ts)}
                                    ref={(elem) => {
                                        if (i === 0 && elem) {
                                            targetItemRefs.current.set(ts, elem);
                                        }
                                    }}
                                    sx={{ cursor: 'pointer' }}
                                />
                                <RelationHorizontalLine />
                            </Box>
                        })}
                    </CardCol>

                    <CardCol width={colWidth}>
                        {ps.targets.map((ts, i) => {
                            return <CardRow key={buildListKey([ts.target, ts.kdInfo])} height={cardHeight}>
                                {ts.commandResults?.slice(0, 4).map((rs, i) => {
                                    return i === 0
                                        ? <CommandResultItem
                                            key={rs.id}
                                            rs={rs}
                                            onSelectCommandResult={() => onSelectCommandResultItem(ps, ts)}
                                            ref={(elem) => {
                                                if (i === 0 && elem) {
                                                    commandResultItemRefs.current.set(ts, elem);
                                                }
                                            }}
                                            sx={{ cursor: 'pointer' }}
                                        />
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

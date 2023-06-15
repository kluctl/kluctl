import { TargetKey } from "../../models";
import { Box, Typography, useTheme } from "@mui/material";
import React, { useCallback, useContext, useMemo, useRef, useState } from "react";
import { AppContext } from "../App";
import { ProjectItem } from "./Projects";
import { TargetItem } from "./Targets";
import Divider from "@mui/material/Divider";
import { CommandResultItem } from "./CommandResultItem";
import { TargetDetailsDrawer } from "./TargetDetailsDrawer";
import { Card, CardCol, cardGap, cardHeight, CardPaper, CardRow, cardWidth, projectCardHeight } from "./Card";
import { TargetSummary } from "../../project-summaries";
import { buildListKey } from "../../utils/listKey";
import { HistoryCards } from "./HistoryCards";
import { useNavigate, useParams } from "react-router-dom";
import { sha256 } from "js-sha256";

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

function mkTargetHash(tk: TargetKey): string {
    return sha256(JSON.stringify(tk));
}

export const TargetsView = () => {
    const theme = useTheme();
    const [selectedTargetSummary, setSelectedTargetSummary] = useState<TargetSummary | undefined>();

    const navigate = useNavigate();
    const { targetKeyHash } = useParams();
    const appContext = useContext(AppContext);
    const projects = appContext.projects;

    const targetsHashes = useMemo(() => {
        const dict = new Map<string, TargetSummary>();
        projects.forEach(ps =>
            ps.targets.forEach(ts =>
                dict.set(mkTargetHash(ts.target), ts)
            )
        );
        return dict;
    }, [projects]);

    const historyCardsTarget = useMemo(() => {
        return targetKeyHash ? targetsHashes.get(targetKeyHash) : undefined;
    }, [targetKeyHash, targetsHashes]);

    const onTargetDetailsDrawerClose = useCallback(() => {
        setSelectedTargetSummary(undefined);
    }, []);

    const onSelectHistoryCardsTarget = useCallback((ts: TargetSummary) => {
        onTargetDetailsDrawerClose();
        navigate(`/targets/${mkTargetHash(ts.target)}`);
    }, [navigate, onTargetDetailsDrawerClose]);

    const onHistoryCardsClose = useCallback(() => {
        navigate(`/targets/`);
    }, [navigate]);

    const cardRefs = useRef<Map<TargetSummary, HTMLElement>>(new Map());
 
    if (historyCardsTarget) {
        const cardElem = cardRefs.current.get(historyCardsTarget);
        const cardRect = cardElem?.getBoundingClientRect();

        return <HistoryCards
            targetSummary={historyCardsTarget}
            initialCardRect={cardRect}
            onClose={onHistoryCardsClose}
        />;
    }

    return <Box minWidth={colWidth * 3} p='0 40px'>
        <TargetDetailsDrawer
            ts={selectedTargetSummary}
            onClose={onTargetDetailsDrawerClose}
        />
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
                        <Card height={projectCardHeight}>
                            <ProjectItem ps={ps} />
                        </Card>
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
                            return <Box key={buildListKey(ts.target)} display='flex'>
                                <Card>
                                    <TargetItem ps={ps} ts={ts}
                                        onSelectTarget={setSelectedTargetSummary} />
                                </Card>
                                <Box flexGrow={1} display='flex' justifyContent='center' alignItems='center' px='9px'>
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
                                </Box>
                            </Box>
                        })}
                    </CardCol>

                    <CardCol width={colWidth}>
                        {ps.targets.map((ts, i) => {
                            return <CardRow key={buildListKey(ts.target)} height={cardHeight}>
                                {ts.commandResults?.map((rs, i) => {
                                    return <Card
                                        key={rs.id}
                                        sx={{
                                            translate: `${-i * (cardWidth + cardGap / 2)}px`,
                                            zIndex: -i,
                                            display: i < 4 ? 'flex' : 'none'
                                        }}
                                        ref={(r: HTMLElement) => {
                                            if (i === 0) {
                                                cardRefs.current.set(ts, r);
                                            }
                                        }}
                                    >
                                        {i === 0
                                            ? <CommandResultItem
                                                ps={ps}
                                                ts={ts}
                                                rs={rs}
                                                onSelectCommandResult={() => onSelectHistoryCardsTarget(ts)}
                                            />
                                            : <CardPaper />
                                        }
                                    </Card>
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

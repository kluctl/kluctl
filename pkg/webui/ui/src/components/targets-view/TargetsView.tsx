import { useLoaderData } from "react-router-dom";
import { CommandResultSummary, ProjectSummary, TargetSummary } from "../../models";
import { Box, BoxProps, Typography, useTheme } from "@mui/material";
import React, { useEffect, useState } from "react";
import { useAppOutletContext } from "../App";
import { api } from "../../api";
import { ProjectItem } from "./Projects";
import { TargetItem } from "./Targets";
import Divider from "@mui/material/Divider";
import { CommandResultItem } from "./CommandResultItem";
import { CommandResultDetailsDrawer } from "./CommandResultDetailsDrawer";
import { TargetDetailsDrawer } from "./TargetDetailsDrawer";
import { RelationHLine } from "../../icons/Icons";

const colWidth = 433;
const cardWidth = 247;
const projectCardHeight = 80;
const cardHeight = 126;
const cardGap = 20;

export async function projectsLoader() {
    const projects = await api.listProjects()
    return projects
}

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

function Card({ children, ...rest }: BoxProps) {
    return <Box display='flex' flexShrink={0} width={cardWidth} height={cardHeight} {...rest}>
        {children}
    </Box>
}

function CardCol({ children, ...rest }: BoxProps) {
    return <Box display='flex' flexDirection='column' gap={`${cardGap}px`} {...rest}>
        {children}
    </Box>
}

function CardRow({ children, ...rest }: BoxProps) {
    return <Box display='flex' gap={`${cardGap}px`} {...rest}>
        {children}
    </Box>
}

const RelationTree = React.memo(({ targetCount }: { targetCount: number }): JSX.Element | null => {
    const theme = useTheme();
    const height = targetCount * cardHeight + (targetCount - 1) * cardGap
    const width = 169;
    const curveRadius = 12;
    const circleRadius = 5;
    const strokeWidth = 2;

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
                <circle
                    key={`circle-${i}`}
                    cx={width - circleRadius - strokeWidth / 2}
                    cy={cy}
                    r={circleRadius}
                    fill={theme.palette.background.default}
                    stroke={theme.palette.secondary.main}
                    strokeWidth={strokeWidth}
                />
            ]
        })}
        <circle
            key='circle-root'
            cx={circleRadius + strokeWidth / 2}
            cy={rootCenterY}
            r={circleRadius}
            fill={theme.palette.background.default}
            stroke={theme.palette.secondary.main}
            strokeWidth={strokeWidth}
        />
    </svg>
});

export const TargetsView = () => {
    const context = useAppOutletContext()
    const [selectedCommandResult, setSelectedCommandResult] = useState<CommandResultSummary>()
    const [selectedTargetSummary, setSelectedTargetSummary] = useState<TargetSummary>()

    const projects = useLoaderData() as ProjectSummary[];

    useEffect(() => {
        context.setFilters(undefined)
    })

    const doSetSelectedCommandResult = (rs?: CommandResultSummary) => {
        setSelectedCommandResult(rs)
        setSelectedTargetSummary(undefined)
    }
    const doSetSelectedTargetSummary = (ts?: TargetSummary) => {
        setSelectedCommandResult(undefined)
        setSelectedTargetSummary(ts)
    }

    return <Box width={"max-content"} p='0 40px'>
        <CommandResultDetailsDrawer rs={selectedCommandResult} onClose={() => setSelectedCommandResult(undefined)} />
        <TargetDetailsDrawer ts={selectedTargetSummary} onClose={() => setSelectedTargetSummary(undefined)} />
        <Box display={"flex"} alignItems={"center"} height='70px'>
            <ColHeader>Projects</ColHeader>
            <ColHeader>Targets</ColHeader>
            <ColHeader>History</ColHeader>
        </Box>
        <Divider />
        {projects.map((ps, i) => {
            return <Box key={i}>
                <Box key={i} display={"flex"} alignItems={"center"} margin='40px 0'>
                    <Box display='flex' alignItems='center' width={colWidth}>
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

                    <CardCol width={colWidth}>
                        {ps.targets.map((ts, i) => {
                            return <Box key={i} display='flex'>
                                <Card>
                                    <TargetItem ps={ps} ts={ts}
                                        onSelectTarget={(ts) => doSetSelectedTargetSummary(ts)} />
                                </Card>
                                <Box flexGrow={1} display='flex' justifyContent='center' alignItems='center'>
                                    <RelationHLine />
                                </Box>
                            </Box>
                        })}
                    </CardCol>

                    <CardCol>
                        {ps.targets.map((ts, i) => {
                            return <CardRow key={i}>
                                {ts.commandResults?.map((rs, i) => {
                                    return <Card key={i}>
                                        <CommandResultItem ps={ps} ts={ts} rs={rs}
                                            onSelectCommandResult={(rs) => doSetSelectedCommandResult(rs)} />
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

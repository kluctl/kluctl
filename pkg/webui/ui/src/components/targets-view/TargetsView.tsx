import { useLoaderData } from "react-router-dom";
import { CommandResultSummary, ProjectSummary, TargetSummary } from "../../models";
import { Box, BoxProps, Typography } from "@mui/material";
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
        <Typography variant={"h6"} textAlign='left' fontSize='20px' fontWeight={700}>{children}</Typography>
    </Box>
}

function Card({ children, ...rest }: BoxProps) {
    return <Box display='flex' flexShrink={0} width={cardWidth} height={cardHeight} {...rest}>
        {children}
    </Box>
}

function CardCol({ children, ...rest }: BoxProps) {
    return <Box display='flex' flexDirection='column' gap='20px' {...rest}>
        {children}
    </Box>
}

function CardRow({ children, ...rest }: BoxProps) {
    return <Box display='flex' gap='20px' {...rest}>
        {children}
    </Box>
}

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

    useEffect(() => {
        if (projects[2].targets.length < 3) {
            projects[2].targets.push(projects[2].targets[0])
        }
    })

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
                    <CardCol width={colWidth}>
                        <Card height={projectCardHeight}>
                            <ProjectItem ps={ps} />
                        </Card>
                    </CardCol>

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

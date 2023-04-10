import { useLoaderData } from "react-router-dom";
import { CommandResultSummary, ProjectSummary, TargetSummary } from "../../models";
import { Box, Typography } from "@mui/material";
import React, { useEffect, useState } from "react";
import { useAppOutletContext } from "../App";
import { api } from "../../api";
import { ProjectItem } from "./Projects";
import { TargetItem } from "./Targets";
import Divider from "@mui/material/Divider";
import { CommandResultItem } from "./CommandResultItem";
import { CommandResultDetailsDrawer } from "./CommandResultDetailsDrawer";
import { TargetDetailsDrawer } from "./TargetDetailsDrawer";

const targetWidth = "220px"
const targetHeight = "110px"

export async function projectsLoader() {
    const projects = await api.listProjects()
    return projects
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

    return <Box width={"max-content"}>
        <CommandResultDetailsDrawer rs={selectedCommandResult} onClose={() => setSelectedCommandResult(undefined)}/>
        <TargetDetailsDrawer ts={selectedTargetSummary} onClose={() => setSelectedTargetSummary(undefined)}/>
        <Box display={"flex"} alignItems={"center"} p={1}>
            <Box minWidth={targetWidth} width={targetWidth}>
                <Typography variant={"h6"} textAlign={"center"}>Projects</Typography>
            </Box>
            <Box minWidth={"20px"}/>
            <Box minWidth={targetWidth} width={targetWidth}>
                <Typography variant={"h6"} textAlign={"center"}>Targets</Typography>
            </Box>
            <Box minWidth={"20px"}/>
            <Box minWidth={targetWidth} width={targetWidth}>
                <Typography variant={"h6"} textAlign={"center"}>History</Typography>
            </Box>
        </Box>
        <Divider/>
        {projects.map((ps, i) => {
            return <Box key={i}>
                <Box display={"flex"} alignItems={"center"} p={1}>
                    <Box minWidth={targetWidth} width={targetWidth}>
                        <Box display={"flex"} minWidth={"220px"} height={targetHeight} p={1}>
                            <ProjectItem ps={ps}/>
                        </Box>
                    </Box>
                    <Box minWidth={"20px"}/>

                    <Box minWidth={targetWidth} width={targetWidth}>
                        {ps.targets.map((ts, i) => {
                            return <Box key={i} display={"flex"} height={targetHeight} p={1}>
                                <TargetItem ps={ps} ts={ts}
                                            onSelectTarget={(ts) => doSetSelectedTargetSummary(ts)}/>
                            </Box>
                        })}
                    </Box>
                    <Box minWidth={"20px"}/>

                    <Box>
                        {ps.targets.map((ts, i) => {
                            return <Box key={i} display={"flex"} height={targetHeight} paddingY={1}>
                                {ts.commandResults?.map((rs, i) => {
                                    return <Box key={i} height={"100%"} paddingX={1}>
                                        <CommandResultItem ps={ps} ts={ts} rs={rs}
                                                           onSelectCommandResult={(rs) => doSetSelectedCommandResult(rs)}/>
                                    </Box>
                                })}
                            </Box>
                        })}
                    </Box>
                </Box>
                <Divider/>
            </Box>
        })}
    </Box>
}

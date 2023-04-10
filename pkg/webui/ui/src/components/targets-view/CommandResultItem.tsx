import { CommandResultSummary, ProjectSummary, TargetSummary } from "../../models";
import { AccountTree, CleaningServices, CloudSync, Delete, Difference } from "@mui/icons-material";
import React, { useEffect, useMemo, useState } from "react";
import * as yaml from "js-yaml";
import { CodeViewer } from "../CodeViewer";
import Paper from "@mui/material/Paper";
import { Box, IconButton, Tooltip, Typography } from "@mui/material";
import { CommandResultStatusLine } from "../result-view/CommandResultStatusLine";
import { useNavigate } from "react-router";
import { formatDurationShort } from "../../utils/duration";

export const CommandResultItem = (props: { ps: ProjectSummary, ts: TargetSummary, rs: CommandResultSummary, onSelectCommandResult: (rs?: CommandResultSummary) => void }) => {
    const calcAgo = () => {
        const t1 = new Date(props.rs.commandInfo.startTime)
        const t2 = new Date()
        const d = t2.getTime() - t1.getTime()
        return formatDurationShort(d)
    }

    const navigate = useNavigate()
    const [ago, setAgo] = useState(calcAgo())

    let Icon = Difference
    switch (props.rs.commandInfo?.command) {
        case "delete":
            Icon = Delete
            break
        case "deploy":
            Icon = CloudSync
            break
        case "diff":
            Icon = Difference
            break
        case "poke-images":
            Icon = CloudSync
            break
        case "prune":
            Icon = CleaningServices
            break
    }

    const cmdInfoYaml = useMemo(() => {
        return yaml.dump(props.rs.commandInfo)
    }, [props.rs])
    let iconTooltip = <CodeViewer code={cmdInfoYaml} language={"yaml"}/>

    useEffect(() => {
        const interval = setInterval(() => setAgo(calcAgo()), 5000);
        return () => clearInterval(interval);
    }, [])

    return <Paper
        elevation={5}
        sx={{width: "100%", height: "100%"}}
        onClick={e => props.onSelectCommandResult(props.rs)}
    >
        <Box display={"flex"} width={"100%"} height={"100%"}>
            <Box display="flex" justifyContent="center" alignItems="center" m={1}>
                <Box display="flex" flexDirection={"column"} alignItems={"center"}>
                    <Tooltip title={iconTooltip}>
                        <Icon fontSize={"large"}/>
                    </Tooltip>
                    <Tooltip title={props.rs.commandInfo.startTime}>
                        <Typography fontSize={"10px"} color={"gray"} align={"center"}>{ago}</Typography>
                    </Tooltip>
                </Box>
            </Box>
            <Box display="flex" flexDirection={"column"} width={"100%"} m={1}>
                <Typography variant={"h6"}>{props.rs.commandInfo?.command}</Typography>
                <Box display={"flex"} marginTop={"auto"}>
                    <Box width={"100%"}>
                        <CommandResultStatusLine rs={props.rs}/>
                    </Box>
                    <Box marginLeft={"auto"} marginBottom={0}>
                        <Box display={"flex"}>
                            <IconButton onClick={e => {
                                e.stopPropagation();
                                navigate(`/results/${props.rs.id}`)

                            }}>
                                <Tooltip title={"Open Result Tree"}>
                                    <AccountTree/>
                                </Tooltip>
                            </IconButton>
                        </Box>
                    </Box>
                </Box>
            </Box>
        </Box>
    </Paper>
}
import React from "react";
import { CommandResultSummary, ProjectSummary, TargetSummary } from "../../models";
import { useEffect, useMemo, useState } from "react";
import * as yaml from "js-yaml";
import { CodeViewer } from "../CodeViewer";
import Paper from "@mui/material/Paper";
import { Box, IconButton, Tooltip, Typography } from "@mui/material";
import { CommandResultStatusLine } from "../result-view/CommandResultStatusLine";
import { useNavigate } from "react-router";
import { formatDurationShort } from "../../utils/duration";
import { DeployIcon, DiffIcon, PruneIcon, TreeViewIcon } from "../../icons/Icons";

const calcAgo = (startTime: string) => {
    const t1 = new Date(startTime)
    const t2 = new Date()
    const d = t2.getTime() - t1.getTime()
    return formatDurationShort(d)
}

export const CommandResultItem = React.memo((props: { 
    ps: ProjectSummary, 
    ts: TargetSummary, 
    rs: CommandResultSummary, 
    onSelectCommandResult: (rs: CommandResultSummary) => void,
    selected?: boolean;
}) => {
    const navigate = useNavigate()
    const [ago, setAgo] = useState(calcAgo(props.rs.commandInfo.startTime))

    let Icon: () => JSX.Element = DiffIcon
    switch (props.rs.commandInfo?.command) {
        case "delete":
            Icon = PruneIcon
            break
        case "deploy":
            Icon = DeployIcon
            break
        case "diff":
            Icon = DiffIcon
            break
        case "poke-images":
            Icon = DeployIcon
            break
        case "prune":
            Icon = PruneIcon
            break
    }

    const cmdInfoYaml = useMemo(() => {
        return yaml.dump(props.rs.commandInfo)
    }, [props.rs])
    let iconTooltip = <CodeViewer code={cmdInfoYaml} language={"yaml"} />

    useEffect(() => {
        const interval = setInterval(() => setAgo(calcAgo(props.rs.commandInfo.startTime)), 5000);
        return () => clearInterval(interval);
    }, [props.rs.commandInfo.startTime])

    return <Paper
        elevation={5}
        sx={{
            width: "100%",
            height: "100%",
            borderRadius: '12px',
            border: '1px solid #59A588',
            boxShadow: '4px 4px 10px #1E617A',
            padding: '20px 16px 5px 16px',
            outline: props.selected ? '8px solid #59A588' : 'none'
        }}
        onClick={e => props.onSelectCommandResult(props.rs)}
    >
        <Box display='flex' flexDirection='column' justifyContent='space-between' height='100%'>
            <Box display='flex' gap='15px'>
                <Tooltip title={iconTooltip}>
                    <Box width='45px' height='45px' flex='0 0 auto' justifyContent='center' alignItems='center'>
                        <Icon />
                    </Box>
                </Tooltip>
                <Box flexGrow={1}>
                    <Typography
                        variant='h6'
                        textAlign='left'
                        textOverflow='ellipsis'
                        overflow='hidden'
                        flexGrow={1}
                    >
                        {props.rs.commandInfo?.command}
                    </Typography>
                    <Tooltip title={props.rs.commandInfo.startTime}>
                        <Typography
                            variant='subtitle1'
                            textAlign='left'
                            textOverflow='ellipsis'
                            overflow='hidden'
                            whiteSpace='nowrap'
                            fontSize='14px'
                            fontWeight={500}
                            lineHeight='19px'
                        >{ago}</Typography>
                    </Tooltip>
                </Box>
            </Box>
            <Box display='flex' alignItems='center' justifyContent='space-between'>
                <Box display='flex' gap='6px' alignItems='center'>
                    <CommandResultStatusLine rs={props.rs} />
                </Box>
                <Box display='flex' gap='6px' alignItems='center' height='39px'>
                    <IconButton
                        onClick={e => {
                            e.stopPropagation();
                            navigate(`/results/${props.rs.id}`);
                        }}
                        sx={{
                            padding: 0,
                            width: 26,
                            height: 26
                        }}
                    >
                        <Tooltip title={"Open Result Tree"}>
                            <Box display='flex'><TreeViewIcon /></Box>
                        </Tooltip>
                    </IconButton>
                </Box>
            </Box>
        </Box>
    </Paper>
});

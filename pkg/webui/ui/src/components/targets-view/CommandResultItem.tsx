import React, { useEffect, useMemo, useState } from "react";
import { CommandResultSummary } from "../../models";
import * as yaml from "js-yaml";
import { CodeViewer } from "../CodeViewer";
import { Box, IconButton, Tooltip, Typography } from "@mui/material";
import { CommandResultStatusLine } from "../result-view/CommandResultStatusLine";
import { useNavigate } from "react-router";
import { DeployIcon, DiffIcon, PruneIcon, TreeViewIcon } from "../../icons/Icons";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { calcAgo } from "../../utils/duration";
import { CardPaper } from "./Card";

export const CommandResultItemHeader = React.memo((props: { rs: CommandResultSummary }) => {
    const { rs } = props;
    const [ago, setAgo] = useState(calcAgo(rs.commandInfo.startTime))

    let Icon: () => JSX.Element = DiffIcon
    switch (rs.commandInfo?.command) {
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

    const iconTooltip = useMemo(() => {
        const cmdInfoYaml = yaml.dump(rs.commandInfo);
        return <CodeViewer code={cmdInfoYaml} language={"yaml"} />
    }, [rs.commandInfo]);

    useEffect(() => {
        const interval = setInterval(() => setAgo(calcAgo(rs.commandInfo.startTime)), 5000);
        return () => clearInterval(interval);
    }, [rs.commandInfo.startTime]);


    return <Box display='flex' gap='15px'>
        <Tooltip title={iconTooltip}>
            <Box width='45px' height='45px' flex='0 0 auto' justifyContent='center' alignItems='center'>
                <Icon />
            </Box>
        </Tooltip>
        <Box>
            <Typography
                variant='h6'
                textAlign='left'
                textOverflow='ellipsis'
                overflow='hidden'
            >
                {rs.commandInfo?.command}
            </Typography>
            <Tooltip title={rs.commandInfo.startTime}>
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
});

export const CommandResultItem = React.memo((props: {
    ps: ProjectSummary,
    ts: TargetSummary,
    rs: CommandResultSummary,
    onSelectCommandResult: (rs: CommandResultSummary) => void,
}) => {
    const { rs, onSelectCommandResult } = props;
    const navigate = useNavigate()

    return <CardPaper
        sx={{ padding: '20px 16px 5px 16px' }}
        onClick={() => onSelectCommandResult(rs)}
    >
        <Box display='flex' flexDirection='column' justifyContent='space-between' height='100%'>
            <CommandResultItemHeader rs={rs} />
            <Box display='flex' alignItems='center' justifyContent='space-between'>
                <Box display='flex' gap='6px' alignItems='center'>
                    <CommandResultStatusLine rs={rs} />
                </Box>
                <Box display='flex' gap='6px' alignItems='center' height='39px'>
                    <IconButton
                        onClick={e => {
                            e.stopPropagation();
                            navigate(`/results/${rs.id}`);
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
    </CardPaper>
});

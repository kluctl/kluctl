import { CommandResultSummary } from "../../models";
import React from "react";
import { DeployIcon, DiffIcon, PruneIcon } from "../../icons/Icons";
import { LiveHelp } from "@mui/icons-material";
import { TargetSummary } from "../../project-summaries";
import Tooltip from "@mui/material/Tooltip";
import { Box, Typography } from "@mui/material";

export const CommandTypeIcon = (props: {ts: TargetSummary, rs: CommandResultSummary, size?: string}) => {
    let icon: React.ReactElement
    let tooltip = props.rs.commandInfo?.command
    const size = props.size || "45px"
    switch (props.rs.commandInfo?.command) {
        default:
            icon = <DiffIcon size={size}/>
            break
        case "delete":
            icon = <PruneIcon size={size}/>
            break
        case "deploy":
            if (props.rs.commandInfo.dryRun) {
                if (props.ts.kd?.deployment.spec.manual && !props.ts.kd?.deployment.spec.dryRun) {
                    icon = <LiveHelp sx={{ width: size, height: size }}/>
                    tooltip = "manual deploy"
                } else {
                    icon = <DeployIcon size={size}/>
                    tooltip = "dry-run deploy"
                }
            } else {
                icon = <DeployIcon size={size}/>
            }
            break
        case "diff":
            icon = <DiffIcon size={size}/>
            break
        case "poke-images":
            icon = <DeployIcon size={size}/>
            break
        case "prune":
            icon = <PruneIcon size={size}/>
            break
    }
    return <Tooltip title={<Typography>{tooltip}</Typography>}>
        <Box>
            {icon}
        </Box>
    </Tooltip>
}
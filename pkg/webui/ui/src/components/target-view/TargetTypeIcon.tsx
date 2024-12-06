import { TargetSummary } from "../../project-summaries";
import React from "react";
import Tooltip from "@mui/material/Tooltip";
import { Box } from "@mui/material";
import { Approval, Toys } from "@mui/icons-material";

export const TargetTypeIcon = (props: {ts: TargetSummary}) => {
    if (props.ts.kd?.deployment.spec.manual) {
        return <Tooltip title={"Deployment needs manual triggering"}>
            <Box display='flex'><Approval/></Box>
        </Tooltip>
    } else if (props.ts.kd?.deployment.spec.dryRun) {
        return <Tooltip title={"Deployment is in dry-run mode"}>
            <Box display='flex'><Toys/></Box>
        </Tooltip>
    } else {
        // dummy icon
        return <Box display='flex'><Toys opacity={"0%"}/></Box>
    }
}
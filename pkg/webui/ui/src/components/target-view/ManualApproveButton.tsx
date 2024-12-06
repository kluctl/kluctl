import { TargetSummary } from "../../project-summaries";
import { useAppContext } from "../App";
import React from "react";
import { RocketLaunch } from "@mui/icons-material";
import { Box, IconButton, Tooltip } from "@mui/material";
import { buildReconcilingState } from "./ReconcilingIcon";

export const ManualApproveButton = (props: {ts: TargetSummary, renderedObjectsHash: string}) => {
    const appCtx = useAppContext()

    if (appCtx.isStatic || !appCtx.user.isAdmin || props.ts.kd?.deployment.spec.dryRun || !props.ts.kd?.deployment.spec.manual) {
        return <></>
    }

    const state = buildReconcilingState(props.ts)
    if (!state) {
        return <></>
    }

    const isApproved = state?.manualObjectsHash === props.renderedObjectsHash

    const handleApproveButton = () => {
        if (!props.ts.kdInfo || !props.ts.kd) {
            return
        }
        if (!props.renderedObjectsHash) {
            return
        }
        if (!state.hasDrift) {
            return
        }
        if (!isApproved) {
            appCtx.api.setManualObjectsHash(props.ts.kdInfo.clusterId, props.ts.kdInfo.name, props.ts.kdInfo.namespace, props.renderedObjectsHash)
        } else {
            appCtx.api.deployNow(props.ts.kdInfo.clusterId, props.ts.kdInfo.name, props.ts.kdInfo.namespace)
        }
    }

    let icon: React.ReactElement
    let tooltip: string
    if (!state.hasDrift) {
        tooltip = "No drift, so there is nothing to be manually deployed!"
        icon = <RocketLaunch color={"disabled"}/>
    } else if (!isApproved) {
        tooltip = "Click here to trigger this manual deployment."
        icon = <RocketLaunch color={"info"}/>
    } else {
        if (state.requestedDeploy) {
            tooltip = "Deployment has been requested, please wait!"
            icon = <RocketLaunch color={"disabled"}/>
        } else {
            tooltip = "Click here to retry this already approved manual deployment."
            icon = <RocketLaunch color={"warning"}/>
        }
    }
    return <Box display='flex' gap='6px' alignItems='center' height='39px'>
        <IconButton
            onClick={e => {
                e.stopPropagation();
                handleApproveButton()
            }}
            sx={{
                padding: 0,
                width: 26,
                height: 26
            }}
        >
            <Tooltip title={tooltip}>
                <Box display='flex'>{icon}</Box>
            </Tooltip>
        </IconButton>
    </Box>
}
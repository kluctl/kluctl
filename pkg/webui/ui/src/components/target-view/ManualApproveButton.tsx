import { TargetSummary } from "../../project-summaries";
import { useAppContext } from "../App";
import React from "react";
import { RocketLaunch } from "@mui/icons-material";
import { Box, IconButton, Tooltip } from "@mui/material";

export const ManualApproveButton = (props: {ts: TargetSummary, renderedObjectsHash: string}) => {
    const appCtx = useAppContext()
    const handleApprove = (approve: boolean) => {
        if (!props.ts.kdInfo || !props.ts.kd) {
            return
        }
        if (approve) {
            if (!props.renderedObjectsHash) {
                return
            }
            appCtx.api.setManualObjectsHash(props.ts.kdInfo.clusterId, props.ts.kdInfo.name, props.ts.kdInfo.namespace, props.renderedObjectsHash)
        } else {
            appCtx.api.setManualObjectsHash(props.ts.kdInfo.clusterId, props.ts.kdInfo.name, props.ts.kdInfo.namespace, "")
        }
    }

    if (appCtx.isStatic || !appCtx.user.isAdmin || props.ts.kd?.deployment.spec.dryRun || !props.ts.kd?.deployment.spec.manual) {
        return <></>
    }

    const hasDrift = !!props.ts.lastDriftDetectionResult?.objects?.length
    const isApproved = props.ts.kd.deployment.spec.manualObjectsHash === props.renderedObjectsHash

    let icon: React.ReactElement
    let tooltip: string
    if (!hasDrift) {
        tooltip = "No drift, so there is nothing to be manually deployed!"
        icon = <RocketLaunch color={"disabled"}/>
    } else if (!isApproved) {
        tooltip = "Click here to trigger this manual deployment."
        icon = <RocketLaunch color={"info"}/>
    } else {
        tooltip = "Click here to cancel this deployment. This will only have an effect if the deployment has not started reconciliation yet!"
        icon = <RocketLaunch color={"success"}/>
    }
    return <Box display='flex' gap='6px' alignItems='center' height='39px'>
        <IconButton
            onClick={e => {
                e.stopPropagation();
                if (hasDrift) {
                    handleApprove(!isApproved)
                }
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
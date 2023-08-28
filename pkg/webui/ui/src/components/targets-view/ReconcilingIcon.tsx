import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { Box, CircularProgress, Typography, useTheme } from "@mui/material";
import { Since } from "../Since";
import React from "react";
import { Done, Error, Hotel, HourglassTop, SyncProblem } from "@mui/icons-material";
import { MessageQuestionIcon } from "../../icons/Icons";
import Tooltip from "@mui/material/Tooltip";

export const ReconcilingIcon = (props: { ps: ProjectSummary, ts: TargetSummary }) => {
    const theme = useTheme();

    if (!props.ts.kd) {
        return <></>
    }

    const buildStateTime = (c: any) => {
        return <>In this state since <Since startTime={c.lastTransitionTime}/></>
    }

    let icon: React.ReactElement | undefined = undefined
    const tooltip: React.ReactNode[] = []

    const annotations: any = props.ts.kd.deployment.metadata?.annotations || {}
    const requestReconcile = annotations["kluctl.io/request-reconcile"]
    const requestValidate = annotations["kluctl.io/request-validate"]
    const requestDeploy = annotations["kluctl.io/request-deploy"]

    if (props.ts.kd.deployment.spec.suspend) {
        icon = <Hotel color={"primary"}/>
        tooltip.push("Deployment is suspended")
    } else if (!props.ts.kd.deployment.status) {
        icon = <MessageQuestionIcon color={theme.palette.warning.main}/>
        tooltip.push("Status not available")
    } else {
        const conditions: any[] | undefined = props.ts.kd.deployment.status.conditions
        const readyCondition = conditions?.find(c => c.type === "Ready")
        const reconcilingCondition = conditions?.find(c => c.type === "Reconciling")

        if (reconcilingCondition) {
            icon = <CircularProgress color={"info"} size={24}/>
            tooltip.push(reconcilingCondition.message)
            tooltip.push(buildStateTime(reconcilingCondition))
        } else {
            if (requestReconcile && requestReconcile !== props.ts.kd.deployment.status.lastHandledReconcileAt) {
                icon = <HourglassTop color={"primary"}/>
                tooltip.push("Reconcile requested: " + requestReconcile)
            } else if (requestValidate && requestValidate !== props.ts.kd.deployment.status.lastHandledValidateAt) {
                icon = <HourglassTop color={"primary"}/>
                tooltip.push("Validate requested: " + requestValidate)
            } else if (requestDeploy && requestDeploy !== props.ts.kd.deployment.status.lastHandledDeployAt) {
                icon = <HourglassTop color={"primary"}/>
                tooltip.push("Deploy requested: " + requestReconcile)
            } else if (readyCondition) {
                if (readyCondition.status === "True") {
                    icon = <Done color={"success"}/>
                    tooltip.push(readyCondition.message)
                } else {
                    if (readyCondition.reason === "PrepareFailed") {
                        icon = <SyncProblem color={"error"}/>
                        tooltip.push(readyCondition.message)
                    } else if (readyCondition.reason === "ValidateFailed") {
                        icon = <Done color={"success"}/>
                        tooltip.push(readyCondition.message)
                    } else {
                        icon = <Error color={"warning"}/>
                        tooltip.push(readyCondition.message)
                    }
                }
                tooltip.push(buildStateTime(readyCondition))
            } else {
                icon = <MessageQuestionIcon color={theme.palette.warning.main}/>
                tooltip.push("Ready condition is missing")
            }
        }
    }

    if (!icon) {
        icon = <MessageQuestionIcon color={theme.palette.warning.main}/>
        tooltip.push("Unexpected state")
    }

    return <Tooltip title={
        <>
            <Typography key={"title"}><b>Reconciliation State</b></Typography>
            {tooltip.map((t, i) => <Typography key={i}>{t}</Typography>)}
        </>
    }>
        <Box display='flex'>{icon}</Box>
    </Tooltip>
}

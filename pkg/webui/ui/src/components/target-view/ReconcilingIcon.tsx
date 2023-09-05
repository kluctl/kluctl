import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { Box, CircularProgress, Typography, useTheme } from "@mui/material";
import { Since } from "../Since";
import React from "react";
import { Done, Error, Hotel, HourglassTop, SyncProblem } from "@mui/icons-material";
import { MessageQuestionIcon } from "../../icons/Icons";
import Tooltip from "@mui/material/Tooltip";

export interface Condition {
    status: string
    reason: string
    message: string
    lastTransitionTime: string
}

export interface ReconcilingState {
    suspended: boolean
    requestedReconcile: string
    requestedValidate: string
    requestedDeploy: string

    statusExists: boolean
    readyCondition?: Condition
    reconcilingCondition?: Condition

    manualObjectsHash?: string
    hasDrift: boolean
    lastDriftDetectionResult?: any
    lastDriftDetectionResultMessage: string
}

export function buildReconcilingState(ts: TargetSummary): ReconcilingState | undefined {
    if (!ts.kd) {
        return undefined
    }
    const annotations: any = ts.kd.deployment.metadata?.annotations || {}
    const requestReconcile = annotations["kluctl.io/request-reconcile"]
    const requestValidate = annotations["kluctl.io/request-validate"]
    const requestDeploy = annotations["kluctl.io/request-deploy"]

    const conditions: any[] | undefined = ts.kd.deployment.status.conditions
    const readyCondition = conditions?.find(c => c.type === "Ready")
    const reconcilingCondition = conditions?.find(c => c.type === "Reconciling")

    const lastDriftDetectionResult: any | undefined = ts.kd.deployment.status?.lastDriftDetectionResult

    return {
        suspended: ts.kd.deployment.spec.suspend,
        requestedReconcile: requestReconcile && requestReconcile !== ts.kd.deployment.status?.lastHandledReconcileAt && requestReconcile,
        requestedValidate: requestValidate && requestValidate !== ts.kd.deployment.status?.lastHandledValidateAt && requestValidate,
        requestedDeploy: requestDeploy && requestDeploy !== ts.kd.deployment.status?.lastHandledDeployAt && requestDeploy,
        statusExists: !!ts.kd.deployment.status,
        readyCondition: readyCondition,
        reconcilingCondition: reconcilingCondition,
        manualObjectsHash: ts.kd.deployment.spec.manualObjectsHash,
        hasDrift: !!lastDriftDetectionResult?.objects?.length,
        lastDriftDetectionResult: lastDriftDetectionResult,
        lastDriftDetectionResultMessage: ts.kd.deployment.status?.lastDriftDetectionResultMessage,
    }
}

export const ReconcilingIcon = (props: { ps: ProjectSummary, ts: TargetSummary }) => {
    const theme = useTheme();

    const state = buildReconcilingState(props.ts)

    if (!state) {
        return <></>
    }

    const buildStateTime = (c: Condition) => {
        return <>In this state since <Since startTime={c.lastTransitionTime}/></>
    }

    let icon: React.ReactElement | undefined = undefined
    const tooltip: React.ReactNode[] = []

    if (state.suspended) {
        icon = <Hotel color={"primary"}/>
        tooltip.push("Deployment is suspended")
    } else if (!state.statusExists) {
        icon = <MessageQuestionIcon color={theme.palette.warning.main}/>
        tooltip.push("Status not available")
    } else {
        if (state.reconcilingCondition) {
            icon = <CircularProgress color={"info"} size={24}/>
            tooltip.push(state.reconcilingCondition.message)
            tooltip.push(buildStateTime(state.reconcilingCondition))
        } else {
            if (state.requestedReconcile) {
                icon = <HourglassTop color={"primary"}/>
                tooltip.push("Reconcile requested: " + state.requestedReconcile)
            } else if (state.requestedValidate) {
                icon = <HourglassTop color={"primary"}/>
                tooltip.push("Validate requested: " + state.requestedValidate)
            } else if (state.requestedDeploy) {
                icon = <HourglassTop color={"primary"}/>
                tooltip.push("Deploy requested: " + state.requestedDeploy)
            } else if (state.readyCondition) {
                if (state.readyCondition.status === "True") {
                    icon = <Done color={"success"}/>
                    tooltip.push(state.readyCondition.message)
                } else {
                    if (state.readyCondition.reason === "PrepareFailed") {
                        icon = <SyncProblem color={"error"}/>
                        tooltip.push(state.readyCondition.message)
                    } else if (state.readyCondition.reason === "ValidateFailed") {
                        icon = <Done color={"success"}/>
                        tooltip.push(state.readyCondition.message)
                    } else {
                        icon = <Error color={"warning"}/>
                        tooltip.push(state.readyCondition.message)
                    }
                }
                tooltip.push(buildStateTime(state.readyCondition))
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

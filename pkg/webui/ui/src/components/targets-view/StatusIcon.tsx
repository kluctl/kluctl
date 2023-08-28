import { ProjectSummary, TargetSummary } from "../../project-summaries";
import React from "react";
import { Box, Typography, useTheme } from "@mui/material";
import { Favorite, HeartBroken } from "@mui/icons-material";
import { MessageQuestionIcon } from "../../icons/Icons";
import { Since } from "../Since";
import Tooltip from "@mui/material/Tooltip";

export const StatusIcon = (props: { ps: ProjectSummary, ts: TargetSummary }) => {
    let icon: React.ReactElement
    const theme = useTheme();

    if (!props.ts.kd?.deployment.spec.validate) {
        icon = <Favorite color={"disabled"}/>
    } else if (props.ts.lastValidateResult === undefined) {
        icon = <MessageQuestionIcon color={theme.palette.error.main}/>
    } else if (props.ts.lastValidateResult.ready && !props.ts.lastValidateResult.errors) {
        if (props.ts.lastValidateResult.warnings) {
            icon = <Favorite color={"warning"}/>
            /*} else if (props.ts.lastValidateResult.drift?.length) {
                icon = <Favorite color={"primary"} />*/
        } else {
            icon = <Favorite color={"success"}/>
        }
    } else {
        icon = <HeartBroken color={"error"}/>
    }

    const tooltip: React.ReactNode[] = []
    if (!props.ts.kd?.deployment.spec.validate) {
        tooltip.push("Validation is disabled.")
    } else if (props.ts.lastValidateResult === undefined) {
        tooltip.push("No validation result available.")
    } else {
        if (props.ts.lastValidateResult.ready && !props.ts.lastValidateResult.errors) {
            tooltip.push("Target is ready.")
        } else {
            tooltip.push("Target is not ready.")
        }
        if (props.ts.lastValidateResult.errors) {
            tooltip.push(`Target has ${props.ts.lastValidateResult.errors} validation errors.`)
        }
        if (props.ts.lastValidateResult.warnings) {
            tooltip.push(`Target has ${props.ts.lastValidateResult.warnings} validation warnings.`)
        }
        /*if (props.ts.lastValidateResult.drift?.length) {
            tooltip.push(`Target has ${props.ts.lastValidateResult.drift.length} drifted objects.`)
        }*/
        tooltip.push(<>Validation performed <Since startTime={new Date(props.ts.lastValidateResult.startTime)}/> ago</>)
    }

    return <Tooltip title={
        <>
            <Typography key={"title"}><b>Validation State</b></Typography>
            {tooltip.map((t, i) => <Typography key={i}>{t}</Typography>)}
        </>
    }>
        <Box display='flex'>{icon}</Box>
    </Tooltip>
}
import { ProjectSummary, TargetSummary } from "../../models";
import { ActionMenuItem, ActionsMenu } from "../ActionsMenu";
import Paper from "@mui/material/Paper";
import { Box, Typography } from "@mui/material";
import React from "react";
import { Jdenticon } from "../Jdenticon";
import Tooltip from "@mui/material/Tooltip";
import { Favorite, Fingerprint, HeartBroken, PublishedWithChanges, QuestionMark } from "@mui/icons-material";
import { api } from "../../api";

const StatusIcon = (props: { ps: ProjectSummary, ts: TargetSummary }) => {
    let icon: React.ReactElement

    if (props.ts.lastValidateResult === undefined) {
        icon = <QuestionMark color={"error"}/>
    } else if (props.ts.lastValidateResult.ready && !props.ts.lastValidateResult.errors) {
        if (props.ts.lastValidateResult.warnings?.length) {
            icon = <Favorite color={"warning"}/>
        } else if (props.ts.lastValidateResult.drift?.length) {
            icon = <Favorite color={"primary"}/>
        } else {
            icon = <Favorite color={"success"}/>
        }
    } else {
        icon = <HeartBroken color={"error"}/>
    }

    const tooltip: string[] = []
    if (props.ts.lastValidateResult === undefined) {
        tooltip.push("No validation result available.")
    } else {
        if (props.ts.lastValidateResult.ready && !props.ts.lastValidateResult.errors?.length) {
            tooltip.push("Target is ready.")
        } else {
            tooltip.push("Target is not ready.")
        }
        if (props.ts.lastValidateResult.errors?.length) {
            tooltip.push(`Target has ${props.ts.lastValidateResult.errors.length} validation errors.`)
        }
        if (props.ts.lastValidateResult.warnings?.length) {
            tooltip.push(`Target has ${props.ts.lastValidateResult.warnings.length} validation warnings.`)
        }
        if (props.ts.lastValidateResult.drift?.length) {
            tooltip.push(`Target has ${props.ts.lastValidateResult.drift.length} drifted objects.`)
        }
    }

    return <Tooltip title={tooltip.map((t, i) => <Typography key={i}>{t}</Typography>)}>
        {icon}
    </Tooltip>
}

export const TargetItem = (props: { ps: ProjectSummary, ts: TargetSummary, onSelectTarget: (ts?: TargetSummary) => void }) => {
    const actionMenuItems: ActionMenuItem[] = []

    actionMenuItems.push({
        icon: <PublishedWithChanges/>,
        text: "Validate now",
        handler: () => {
            api.validateNow(props.ps.project, props.ts.target)
        }
    })

    const allContexts: string[] = []

    const handleContext = (c?: string) => {
        if (!c) {
            return
        }
        if (allContexts.find(x => x === c)) {
            return
        }
        allContexts.push(c)
    }

    props.ts.commandResults?.forEach(rs => {
        handleContext(rs.commandInfo.contextOverride)
        handleContext(rs.target.context)
    })

    const contextTooltip = <Box textAlign={"center"}>
        <Typography>All known contexts:</Typography>
        {allContexts.map((context, i) => (
            <Typography key={i}>{context}</Typography>
        ))}
    </Box>

    let targetName = props.ts.target.targetName
    if (!targetName) {
        targetName = "<no-name>"
    }

    return <Paper
        elevation={5}
        sx={{ width: "100%", height: "100%" }}
        onClick={e => props.onSelectTarget(props.ts)}
    >
        <Box display="flex" flexDirection={"column"} justifyContent={"space-between"} height={"100%"}>
            <Box p={1} paddingBottom={0}>
                <Typography
                    variant={"subtitle2"}
                    textAlign={"center"}
                    textOverflow={"ellipsis"}
                    overflow={"hidden"}
                    whiteSpace={"nowrap"}
                >
                    {targetName}
                </Typography>
                {allContexts.length ?
                    <Tooltip title={contextTooltip}>
                        <Typography
                            variant={"subtitle1"}
                            textAlign={"center"}
                            textOverflow={"ellipsis"}
                            overflow={"hidden"}
                            whiteSpace={"nowrap"}
                            fontSize={"10px"}
                        >
                            {allContexts[0]}
                        </Typography>

                    </Tooltip>
                    : <></>}
            </Box>
            <Box>
                <Box display={"flex"} alignItems={"center"}>
                    <Box display={"flex"} width={"100%"}>
                        <Tooltip title={"Cluster ID: " + props.ts.target.clusterId}>
                            <div>
                                <Jdenticon value={props.ts.target.clusterId} size={"24px"}/>
                            </div>
                        </Tooltip>
                        <Tooltip title={"Discriminator: " + props.ts.target.discriminator}>
                            <div>
                                <Fingerprint/>
                            </div>
                        </Tooltip>
                    </Box>
                    <StatusIcon {...props}/>
                    <Box marginLeft={"auto"} alignItems={"center"}>
                        <ActionsMenu menuItems={actionMenuItems}/>
                    </Box>
                </Box>
            </Box>
        </Box>
    </Paper>
}

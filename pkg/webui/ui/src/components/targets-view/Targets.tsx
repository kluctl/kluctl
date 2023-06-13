import { KluctlDeploymentInfo } from "../../models";
import { ActionMenuItem, ActionsMenu } from "../ActionsMenu";
import { Box, Typography, useTheme } from "@mui/material";
import React, { useContext } from "react";
import Tooltip from "@mui/material/Tooltip";
import { Favorite, HeartBroken, PublishedWithChanges } from "@mui/icons-material";
import { CpuIcon, FingerScanIcon, MessageQuestionIcon, TargetIcon } from "../../icons/Icons";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { calcAgo } from "../../utils/duration";
import { ApiContext } from "../App";
import { CardPaper } from "./Card";

const StatusIcon = (props: { ps: ProjectSummary, ts: TargetSummary }) => {
    let icon: React.ReactElement
    const theme = useTheme();

    if (props.ts.lastValidateResult === undefined) {
        icon = <MessageQuestionIcon color={theme.palette.error.main} />
    } else if (props.ts.lastValidateResult.ready && !props.ts.lastValidateResult.errors) {
        if (props.ts.lastValidateResult.warnings?.length) {
            icon = <Favorite color={"warning"} />
        } else if (props.ts.lastValidateResult.drift?.length) {
            icon = <Favorite color={"primary"} />
        } else {
            icon = <Favorite color={"success"} />
        }
    } else {
        icon = <HeartBroken color={"error"} />
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
        tooltip.push("Validation performed " + calcAgo(props.ts.lastValidateResult.startTime) + " ago")
    }

    return <Tooltip title={tooltip.map(t => <Typography key={t}>{t}</Typography>)}>
        <Box display='flex'>{icon}</Box>
    </Tooltip>
}

export const TargetItem = (props: { ps: ProjectSummary, ts: TargetSummary, onSelectTarget: (ts?: TargetSummary) => void }) => {
    const api = useContext(ApiContext)
    const actionMenuItems: ActionMenuItem[] = []

    actionMenuItems.push({
        icon: <PublishedWithChanges />,
        text: "Validate now",
        handler: () => {
            api.validateNow(props.ps.project, props.ts.target)
        }
    })

    let kd: KluctlDeploymentInfo | undefined
    let allKdEqual = true
    props.ts.commandResults?.forEach(rs => {
        if (rs.commandInfo.kluctlDeployment) {
            if (!kd) {
                kd = rs.commandInfo.kluctlDeployment
            } else {
                if (kd.name !== rs.commandInfo.kluctlDeployment.name || kd.namespace !== rs.commandInfo.kluctlDeployment.namespace) {
                    allKdEqual = false
                }
            }
        }
    })

    if (kd && allKdEqual) {
        actionMenuItems.push({
            icon: <PublishedWithChanges />,
            text: "Reconcile",
            handler: () => {
                api.reconcileNow(props.ts.target.clusterId, kd!.name, kd!.namespace)
            }
        })
        actionMenuItems.push({
            icon: <PublishedWithChanges />,
            text: "Deploy",
            handler: () => {
                api.deployNow(props.ts.target.clusterId, kd!.name, kd!.namespace)
            }
        })
    }

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
        <Typography variant="subtitle2">All known contexts:</Typography>
        {allContexts.map(context => (
            <Typography key={context} variant="subtitle2">{context}</Typography>
        ))}
    </Box>

    let targetName = props.ts.target.targetName
    if (!targetName) {
        targetName = "<no-name>"
    }

    return <CardPaper
        sx={{ padding: '20px 16px 12px 16px' }}
        onClick={e => props.onSelectTarget(props.ts)}
    >
        <Box display='flex' flexDirection='column' justifyContent='space-between' height='100%'>
            <Box display='flex' gap='15px'>
                <Box flexShrink={0}><TargetIcon /></Box>
                <Box flexGrow={1}>
                    <Typography
                        variant='h6'
                        textAlign='left'
                        textOverflow='ellipsis'
                        overflow='hidden'
                        flexGrow={1}
                    >
                        {targetName}
                    </Typography>
                    {allContexts.length ?
                        <Tooltip title={contextTooltip}>
                            <Typography
                                variant='subtitle1'
                                textAlign='left'
                                textOverflow='ellipsis'
                                overflow='hidden'
                                whiteSpace='nowrap'
                                fontSize='14px'
                                fontWeight={500}
                                lineHeight='19px'
                            >
                                {allContexts[0]}
                            </Typography>
                        </Tooltip>
                        : <></>}
                </Box>
            </Box>
            <Box display='flex' alignItems='center' justifyContent='space-between'>
                <Box display='flex' gap='6px' alignItems='center'>
                    <Tooltip title={"Cluster ID: " + props.ts.target.clusterId}>
                        <Box display='flex'><CpuIcon /></Box>
                    </Tooltip>
                    <Tooltip title={"Discriminator: " + props.ts.target.discriminator}>
                        <Box display='flex'><FingerScanIcon /></Box>
                    </Tooltip>
                </Box>
                <Box display='flex' gap='6px' alignItems='center'>
                    <StatusIcon {...props} />
                    <ActionsMenu menuItems={actionMenuItems} />
                </Box>
            </Box>
        </Box>
    </CardPaper>
}

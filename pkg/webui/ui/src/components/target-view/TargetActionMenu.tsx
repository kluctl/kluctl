import { TargetSummary } from "../../project-summaries";
import { useAppContext } from "../App";
import { ActionMenuItem, ActionsMenu } from "../ActionsMenu";
import { Pause, PlayArrow, PublishedWithChanges, RocketLaunch, Troubleshoot } from "@mui/icons-material";
import { Typography } from "@mui/material";
import React, { useMemo } from "react";

export const TargetActionMenu = (props: {ts: TargetSummary}) => {
    const appCtx = useAppContext()
    const kd = props.ts.kd

    const actionMenuItems = useMemo(() => {
        const actionMenuItems: ActionMenuItem[] = []
        if (!kd || !appCtx.user.isAdmin) {
            return actionMenuItems
        }

        actionMenuItems.push({
            icon: <PublishedWithChanges/>,
            text: <Typography>Reconcile</Typography>,
            handler: () => {
                appCtx.api.reconcileNow(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace)
            }
        })
        actionMenuItems.push({
            icon: <Troubleshoot/>,
            text: <Typography>Validate</Typography>,
            handler: () => {
                appCtx.api.validateNow(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace)
            }
        })
        actionMenuItems.push({
            icon: <RocketLaunch/>,
            text: <Typography>Deploy</Typography>,
            handler: () => {
                appCtx.api.deployNow(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace)
            }
        })
        if (!kd.deployment.spec.suspend) {
            actionMenuItems.push({
                icon: <Pause/>,
                text: <Typography>Suspend</Typography>,
                handler: () => {
                    appCtx.api.setSuspended(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace, true)
                }
            })
        } else {
            actionMenuItems.push({
                icon: <PlayArrow/>,
                text: <Typography>Resume</Typography>,
                handler: () => {
                    appCtx.api.setSuspended(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace, false)
                }
            })
        }

        return actionMenuItems
    }, [props.ts, appCtx.user])

    return <ActionsMenu menuItems={actionMenuItems}/>
}
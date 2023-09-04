import Tooltip from "@mui/material/Tooltip";
import { TargetSummary } from "../../project-summaries";
import { Box, Typography } from "@mui/material";
import React, { useMemo } from "react";
import { Jdenticon } from "../Jdenticon";

export const gatherContexts = (ts: TargetSummary) => {
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

    ts.commandResults?.forEach(rs => {
        handleContext(rs.commandInfo.contextOverride)
        handleContext(rs.target.context)
    })

    return allContexts
}

export const ClusterIcon = (props: {ts: TargetSummary}) => {
    const iconTooltip = useMemo(() => {
        const allContexts = gatherContexts(props.ts)
        const children: React.ReactNode[] = []
        children.push(<Typography key={children.length} variant="subtitle2"><b>Cluster ID</b></Typography>)
        children.push(<Typography key={children.length} variant="subtitle2">{props.ts.target.clusterId}</Typography>)
        children.push(<br key={children.length}/>)

        if (allContexts.length) {
            children.push(<Typography key={children.length} variant="subtitle2"><b>Contexts</b></Typography>)
            allContexts.forEach(context => {
                children.push(<Typography key={children.length} variant="subtitle2">{context}</Typography>)
            })
        }
        const iconTooltip = <Box textAlign={"center"}>
            {children}
        </Box>
        return iconTooltip
    }, [props.ts])

    return <Tooltip title={iconTooltip}>
        <Box>
            <Jdenticon value={props.ts.target.clusterId} size={"48px"}/>
        </Box>
    </Tooltip>
}
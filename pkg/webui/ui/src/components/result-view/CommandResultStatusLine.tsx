import { CommandResultSummary, ValidateResult } from "../../models";
import { Box, Tooltip, Typography } from "@mui/material";
import { Dangerous, Delete, Difference, LibraryAdd, LinkOff, Warning } from "@mui/icons-material";
import React from "react";

export interface StatusLineProps {
    errors?: number
    warnings?: number
    changedObjects?: number
    newObjects?: number
    deletedObjects?: number
    orphanObjects?: number
}

export const StatusLine = (props: StatusLineProps) => {
    const children: React.ReactElement[] = []

    const doPush = (n: number | undefined, t: string, icon: React.ReactElement) => {
        if (n) {
            children.push(
                <Box key={children.length} display={"flex"} flexDirection={"column"}>
                    <Tooltip title={n + " " + t}>
                        {icon}
                    </Tooltip>
                    <Typography fontSize={"10px"} align={"center"}>{n}</Typography>
                </Box>
            )
        }
    }

    doPush(props.errors, "total errors.", <Dangerous color={"error"}/>)
    doPush(props.warnings, "total warnings.", <Warning color={"warning"}/>)
    doPush(props.newObjects, "new objects.", <LibraryAdd color={"info"}/>)
    doPush(props.deletedObjects, "deleted objects.", <Delete color={"info"}/>)
    doPush(props.orphanObjects, "orphan objects.", <LinkOff color={"info"}/>)
    doPush(props.changedObjects, "changed objects.", <Difference color={"info"}/>)

    return <Box display="flex" width={"100%"}>
        {children}
    </Box>
}

export const CommandResultStatusLine = (props: { rs: CommandResultSummary }) => {
    return <StatusLine errors={props.rs.errors}
                       warnings={props.rs.warnings}
                       changedObjects={props.rs.changedObjects}
                       newObjects={props.rs.newObjects}
                       deletedObjects={props.rs.deletedObjects}
                       orphanObjects={props.rs.orphanObjects}
    />
}


export const ValidateResultStatusLine = (props: { vr?: ValidateResult }) => {
    return <StatusLine errors={props.vr?.errors?.length}
                       warnings={props.vr?.warnings?.length}
                       changedObjects={props.vr?.drift?.length}
    />
}
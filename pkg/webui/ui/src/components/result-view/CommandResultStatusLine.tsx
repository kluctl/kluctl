import { CommandResultSummary, ValidateResult } from "../../models";
import { Box, Tooltip, Typography } from "@mui/material";
import React from "react";
import { AddedIcon, ChangedIcon, ErrorIcon, OrphanIcon, TrashIcon, WarningIcon } from "../../icons/Icons";

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
                        <Box
                            display='flex'
                            alignItems='center'
                            justifyContent='center'
                            width='24px'
                            height='24px'
                        >
                            {icon}
                        </Box>
                    </Tooltip>
                    <Typography fontSize={"10px"} align={"center"}>{n}</Typography>
                </Box>
            )
        }
    }

    doPush(props.errors, "total errors.", <ErrorIcon />)
    doPush(props.warnings, "total warnings.", <WarningIcon />)
    doPush(props.newObjects, "new objects.", <AddedIcon />)
    doPush(props.deletedObjects, "deleted objects.", <TrashIcon />)
    doPush(props.orphanObjects, "orphan objects.", <OrphanIcon />)
    doPush(props.changedObjects, "changed objects.", <ChangedIcon />)

    return <Box display="flex" width={"100%"}>
        {children}
    </Box>
}

export const CommandResultStatusLine = (props: { rs: CommandResultSummary }) => {
    return <StatusLine errors={props.rs.errors?.length}
        warnings={props.rs.warnings?.length}
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
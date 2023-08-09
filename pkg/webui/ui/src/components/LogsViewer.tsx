import React, { useContext, useEffect, useState } from "react";
import { ApiContext } from "./App";
import { Box } from "@mui/material";

export function LogsViewer(props: {cluster?: string, name?: string, namespace?: string, reconcileId?: string}) {
    const api = useContext(ApiContext);
    let [lines, setLines] = useState<React.ReactElement[]>([])

    useEffect(() => {
        console.log(props)
        const cancel = api.watchLogs(props.cluster, props.name, props.namespace, props.reconcileId, (jsonLines: any[]) => {
            jsonLines.forEach(l => {
                lines = [...lines, <div key={lines.length}>{l.msg}<br/></div>]
                setLines(lines)
            })
        })
        return cancel
    }, [props.reconcileId])

    return <Box height={"100%"}>
        {lines}
    </Box>
}

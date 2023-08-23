import React, { useEffect, useState } from "react";
import { useAppContext } from "./App";
import { Box } from "@mui/material";
import { VariableSizeList as List } from 'react-window';
import AutoSizer from "react-virtualized-auto-sizer";

export function LogsViewer(props: { cluster?: string, name?: string, namespace?: string, reconcileId?: string }) {
    const appCtx = useAppContext();
    const [lines, setLines] = useState<any[]>([])

    useEffect(() => {
        let lines: any[] = []

        const appendLines = (l: any[]) => {
            lines = [...lines, ...l]
            setLines(lines)
        }

        const cancel = appCtx.api.watchLogs(props.cluster, props.name, props.namespace, props.reconcileId, (jsonLines: any[]) => {
            appendLines(jsonLines)
        })
        return cancel
    }, [props.cluster, props.name, props.namespace, props.reconcileId, appCtx.api])

    const lineHeight = 25

    const Row = (props: { index: number, style: any }) => {
        const s = {
            ...props.style,
            whiteSpace: "nowrap"
        }
        return <div style={s}>{lines[props.index].msg}</div>
    }

    return <Box width={"100%"} flex={"1 1 auto"}>
        <AutoSizer>
            {(props: { height: number, width: number }) => {
                const getItemSize = (index: number) => {
                    const l: string = lines[index].msg.split("\n")
                    return l.length * lineHeight
                }

                return <List
                    itemCount={lines.length}
                    itemSize={getItemSize}
                    width={props.width}
                    height={props.height}
                >
                    {Row}
                </List>
            }}
        </AutoSizer>
    </Box>
}

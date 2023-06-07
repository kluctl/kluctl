import { CommandResultSummary } from "../../models";
import { getApi, usePromise } from "../../api";
import { NodeBuilder } from "../result-view/nodes/NodeBuilder";
import React, { Suspense, useEffect, useState } from "react";
import { NodeData } from "../result-view/nodes/NodeData";
import { SidePanel } from "../result-view/SidePanel";
import { Box, Drawer, ThemeProvider } from "@mui/material";
import { Loading } from "../Loading";
import { dark } from "../theme";

async function doGetRootNode(rs: CommandResultSummary) {
    const api = await getApi()
    const shortNames = api.getShortNames()
    const r = api.getResult(rs.id)
    const builder = new NodeBuilder({
        shortNames: await shortNames,
        summary: rs,
        commandResult: await r,
    })
    const [node, nodeMap] = builder.buildRoot()
    return node
}

export const CommandResultDetailsDrawer = (props: { rs?: CommandResultSummary, onClose: () => void }) => {
    const [prevId, setPrevId] = useState<string>()
    const [promise, setPromise] = useState<Promise<NodeData>>(new Promise(() => undefined))

    useEffect(() => {
        if (props.rs === undefined) {
            return
        }
        if (props.rs.id === prevId) {
            return
        }
        setPrevId(props.rs.id)
        setPromise(doGetRootNode(props.rs))
    }, [props.rs])

    const Content = (props: { onClose: () => void }) => {
        const node = usePromise(promise)
        return <SidePanel provider={node} onClose={props.onClose} />
    }

    return <ThemeProvider theme={dark}>
        <Drawer
            sx={{ zIndex: 1300 }}
            anchor={"right"}
            open={props.rs !== undefined}
            onClose={props.onClose}
            ModalProps={{ BackdropProps: { invisible: true } }}
        >
            <Box width={"720px"} height={"100%"}>
                <Suspense fallback={<Loading />}>
                    <Content onClose={props.onClose} />
                </Suspense>
            </Box>
        </Drawer>
    </ThemeProvider>
}
import { CommandResultSummary } from "../../models";
import { api, usePromise } from "../../api";
import { NodeBuilder } from "../result-view/nodes/NodeBuilder";
import React, { Suspense, useEffect, useState } from "react";
import { NodeData } from "../result-view/nodes/NodeData";
import { SidePanel } from "../result-view/SidePanel";
import { Box, Drawer } from "@mui/material";
import { Loading } from "../Loading";

async function doGetRootNode(rs: CommandResultSummary) {
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

    const Content = () => {
        const node = usePromise(promise)
        return <SidePanel provider={node}/>
    }

    return <Drawer
        sx={{ zIndex: 1300 }}
        anchor={"right"}
        open={props.rs !== undefined}
        onClose={() => props.onClose()}
    >
        <Box width={"800px"} height={"100%"}>
            <Suspense fallback={<Loading/>}>
                <Content/>
            </Suspense>
        </Box>
    </Drawer>
}
import { CommandResultSummary, ProjectSummary, TargetSummary } from "../../models";
import { getApi, usePromise } from "../../api";
import { NodeBuilder } from "../result-view/nodes/NodeBuilder";
import { Suspense, useEffect, useState } from "react";
import { NodeData } from "../result-view/nodes/NodeData";
import { SidePanel } from "../result-view/SidePanel";
import { Box, Drawer, ThemeProvider } from "@mui/material";
import { Loading } from "../Loading";
import { dark, light } from "../theme";
import { Card, CardCol } from "./Card";
import { CommandResultItem } from "./CommandResultItem";

const sidePanelWidth = 720;

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

export const CommandResultDetailsDrawer = (props: { rs?: CommandResultSummary, ts?: TargetSummary, ps?: ProjectSummary, onClose: () => void }) => {
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
    }, [props.rs, prevId])

    const Content = (props: { onClose: () => void }) => {
        const node = usePromise(promise)
        return <SidePanel provider={node} onClose={props.onClose} />
    }

    const { ps, ts } = props;

    return <ThemeProvider theme={dark}>
        <Drawer
            sx={{ zIndex: 1300 }}
            anchor={"right"}
            open={props.rs !== undefined}
            onClose={props.onClose}
        >
            <Box width={sidePanelWidth} height={"100%"}>
                <Suspense fallback={<Loading />}>
                    <Content onClose={props.onClose} />
                </Suspense>
            </Box>
            <ThemeProvider theme={light}>
                <Box
                    sx={{
                        position: 'fixed',
                        top: 0,
                        bottom: 0,
                        left: 0,
                        right: sidePanelWidth,
                        padding: '30px',
                        overflow: 'auto',
                        display: 'flex',
                        justifyContent: 'center'
                    }}
                >
                    <CardCol>
                        {ps && ts?.commandResults?.map((rs, i) => {
                            return <Card key={i} >
                                <CommandResultItem
                                    ps={ps}
                                    ts={ts}
                                    rs={rs}
                                    onSelectCommandResult={() => { }}
                                />
                            </Card>
                        })}
                    </CardCol>
                </Box>
            </ThemeProvider>
        </Drawer>
    </ThemeProvider>
}
import { CommandResultSummary } from "../../models";
import { api, usePromise } from "../../api";
import { NodeBuilder } from "../result-view/nodes/NodeBuilder";
import { Suspense, useEffect, useState } from "react";
import { NodeData } from "../result-view/nodes/NodeData";
import { SidePanel } from "../result-view/SidePanel";
import { Box, Drawer, ThemeProvider, useTheme } from "@mui/material";
import { Loading } from "../Loading";
import { dark } from "../theme";
import { Card, CardCol } from "./Card";
import { CommandResultItem } from "./CommandResultItem";
import React from "react";
import { ProjectSummary, TargetSummary } from "../../project-summaries";

const sidePanelWidth = 720;

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

export const CommandResultDetailsDrawer = React.memo((props: {
    rs?: CommandResultSummary,
    ts?: TargetSummary,
    ps?: ProjectSummary,
    onClose: () => void
}) => {
    const { ps, ts } = props;
    const theme = useTheme();
    const [promise, setPromise] = useState<Promise<NodeData>>(new Promise(() => undefined));
    const [selectedCommandResult, setSelectedCommandResult] = useState<CommandResultSummary | undefined>();
    const [prevTargetSummary, setPrevTargetSummary] = useState<TargetSummary | undefined>(ts);

    if (prevTargetSummary !== ts) {
        setPrevTargetSummary(ts);
        setSelectedCommandResult(ts?.commandResults?.[0]);
    }

    useEffect(() => {
        if (selectedCommandResult === undefined) {
            return
        }
        setPromise(doGetRootNode(selectedCommandResult));
    }, [selectedCommandResult])

    const Content = (props: { onClose: () => void }) => {
        const node = usePromise(promise)
        return <SidePanel provider={node} onClose={props.onClose} />
    }

    return <>
        {ps && ts &&
            <Box
                sx={{
                    position: 'fixed',
                    top: 0,
                    bottom: 0,
                    right: sidePanelWidth,
                    width: `calc(100% - ${sidePanelWidth}px)`,
                    overflow: 'auto',
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    padding: '25px 0',
                    zIndex: theme.zIndex.modal + 1
                }}
                onClick={props.onClose}
            >
                <CardCol onClick={(e) => e.stopPropagation()} flexGrow={1} justifyContent='center'>
                    {ts.commandResults?.map((rs, i) => {
                        return <Card key={i}>
                            <CommandResultItem
                                ps={ps}
                                ts={ts}
                                rs={rs}
                                onSelectCommandResult={setSelectedCommandResult}
                                selected={rs === selectedCommandResult}
                            />
                        </Card>
                    })}
                </CardCol>
            </Box>
        }
        <ThemeProvider theme={dark}>
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
            </Drawer>
        </ThemeProvider>
    </>
});

import React, { useContext, useEffect, useState } from "react";
import { CommandResultSummary } from "../../models";
import { ApiContext, AppContext } from "../App";
import { Loading } from "../Loading";
import { ErrorMessage } from "../ErrorMessage";
import { doGetRootNode } from "./CommandResultCard";
import { Box, Divider } from "@mui/material";
import { CommandResultNodeData } from "./nodes/CommandResultNode";
import { NodeData } from "./nodes/NodeData";
import CommandResultTree from "./CommandResultTree";
import { SidePanel } from "./SidePanel";

export const CommandResultBody = React.memo((props: {
    rs: CommandResultSummary,
    loadData?: boolean
}) => {
    const api = useContext(ApiContext);
    const appContext = useContext(AppContext);

    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<any>();
    const [node, setNode] = useState<CommandResultNodeData>();
    const [prevRsId, setPrevRsId] = useState<string>();

    const [selectedNode, setSelectedNode] = useState<NodeData | undefined>();

    useEffect(() => {
        let cancelled = false;
        if (!props.loadData) {
            return;
        }

        if (prevRsId === props.rs.id) {
            return;
        }

        const doStartLoading = async () => {
            try {
                setLoading(true);
                const n = await doGetRootNode(api, props.rs, appContext.shortNames);
                if (cancelled) {
                    return;
                }
                setNode(n);
                setPrevRsId(props.rs.id);
            } catch (error) {
                setError(error);
            }
            setLoading(false);
        };

        doStartLoading();

        return () => {
            cancelled = true;
        }
    }, [api, appContext.shortNames, prevRsId, props.loadData, props.rs])

    if (loading) {
        return <Loading />;
    }

    if (error) {
        return <ErrorMessage>
            {error.message}
        </ErrorMessage>;
    }

    if (!node) {
        return null;
    }

    return <Box display={"flex"} width={"100%"} height={"100%"}>
        <Box overflow={"auto"} width={"100%"} height={"100%"}>
            <CommandResultTree
                rootNode={node}
                onSelectNode={setSelectedNode}
                //activeFilters={context.filters}
            />
        </Box>
        <Divider orientation={"vertical"} sx={{marginX: "10px"}}/>
        <Box minWidth={"50%"} maxWidth={"50%"} height={"100%"}>
            <SidePanel provider={selectedNode} onClose={() => setSelectedNode(undefined)} />
        </Box>
    </Box>
});

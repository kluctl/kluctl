import React, { useMemo, useState } from "react";
import { CommandResult } from "../../models";
import { Box, Divider } from "@mui/material";
import { NodeData } from "./nodes/NodeData";
import CommandResultTree from "./CommandResultTree";
import { SidePanel } from "./SidePanel";
import { NodeBuilder } from "./nodes/NodeBuilder";

export const CommandResultBody = React.memo((props: {
    cr: CommandResult,
}) => {
    const [selectedNode, setSelectedNode] = useState<NodeData | undefined>();

    const node = useMemo(() => {
        const builder = new NodeBuilder(props.cr)
        const [node] =  builder.buildRoot()
        return node
    }, [props.cr])

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

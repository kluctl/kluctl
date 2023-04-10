import * as React from 'react';
import { useEffect, useMemo, useState } from 'react';
import TreeView from '@mui/lab/TreeView';
import { TreeItem } from "@mui/lab";
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import { NodeBuilder } from "./nodes/NodeBuilder";
import { NodeData } from "./nodes/NodeData";
import { ActiveFilters, FilterNode } from "./NodeStatusFilter";
import { CommandResultProps } from "./CommandResultView";
import { Loading } from "../Loading";

export interface CommandResultTreeProps {
    commandResultProps?: CommandResultProps

    onSelectNode: (node?: NodeData) => void
    activeFilters?: ActiveFilters
}

const CommandResultTree = (props: CommandResultTreeProps) => {
    const [expanded, setExpanded] = useState<string[]>(["root"]);
    const [selectedNodeId, setSelectedNodeId] = useState<string>()

    const [rootNode, nodeMap] = useMemo(() => {
        if (!props.commandResultProps) {
            return [undefined, undefined]
        }

        const builder = new NodeBuilder(props.commandResultProps)
        return builder.buildRoot()
    }, [props.commandResultProps])

    const handleToggle = (event: React.SyntheticEvent, nodeIds: string[]) => {
        setExpanded(nodeIds);
    };

    const handleDoubleClick = (e: React.SyntheticEvent, node: NodeData) => {
        if (expanded.includes(node.id)) {
            setExpanded(expanded.filter((item) => item !== node.id));
        } else {
            setExpanded([...expanded, node.id]);
        }
        e.stopPropagation()
    };

    const handleItemClick = (e: React.SyntheticEvent, node: NodeData) => {
        setSelectedNodeId(node.id)
        e.stopPropagation()
    }

    const onSelectNode = props.onSelectNode
    useEffect(() => {
        if (!nodeMap || !selectedNodeId) {
            return
        }
        const node = nodeMap.get(selectedNodeId)
        if (!node) {
            setSelectedNodeId(undefined)
        }
        onSelectNode(node)
    }, [nodeMap, selectedNodeId, onSelectNode])

    const renderTree = (nodes: NodeData) => {
        if (!FilterNode(nodes, props.activeFilters)) {
            return null
        }
        return <TreeItem
            key={nodes.id}
            nodeId={nodes.id}
            label={<div onClick={(e: React.SyntheticEvent) => handleItemClick(e, nodes)}>{nodes.buildTreeItem()}</div>}
            sx={{ marginBottom: "5px", marginTop: "5px" }}
            onDoubleClick={(e: React.SyntheticEvent) => handleDoubleClick(e, nodes)}
        >
            {Array.isArray(nodes.children)
                ? nodes.children.map((node) => renderTree(node))
                : null}
        </TreeItem>
    };

    if (!rootNode) {
        return <Loading/>
    }

    return <TreeView expanded={expanded}
                     onNodeToggle={handleToggle}
                     aria-label="rich object"
                     defaultCollapseIcon={<ExpandMoreIcon/>}
                     defaultExpandIcon={<ChevronRightIcon/>}
                     sx={{ width: "100%" }}
    >
        {renderTree(rootNode)}
    </TreeView>
}

export default CommandResultTree;

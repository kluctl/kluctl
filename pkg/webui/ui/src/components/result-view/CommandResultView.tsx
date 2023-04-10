import * as React from 'react';
import { useEffect, useState } from 'react';
import { Box, Divider } from "@mui/material";
import { CommandResult, CommandResultSummary, ShortName } from "../../models";
import { NodeData } from "./nodes/NodeData";
import { SidePanel } from "./SidePanel";
import { ActiveFilters, NodeStatusFilter } from "./NodeStatusFilter";
import CommandResultTree from "./CommandResultTree";
import { useLoaderData } from "react-router-dom";
import { api } from "../../api";
import { useAppOutletContext } from "../App";

export interface CommandResultProps {
    shortNames: ShortName[]
    summary: CommandResultSummary
    commandResult: CommandResult
}

export async function commandResultLoader({ params }: any) {
    const result = api.getResult(params.id)
    const shortNames = api.getShortNames()
    const summaries = api.listResults()

    return {
        shortNames: await shortNames,
        summary: (await summaries).find(x => x.id === params.id),
        commandResult: await result,
    }
}

export const CommandResultView = () => {
    const context = useAppOutletContext()
    const commandResultProps = useLoaderData() as CommandResultProps
    const [activeFilters, setActiveFilters] = useState<ActiveFilters>()
    const [sidePanelNode, setSidePanelNode] = useState<NodeData | undefined>()

    useEffect(() => {
        context.setFilters(<>
            <NodeStatusFilter onFilterChange={setActiveFilters}/>
        </>)
    }, [])

    return <Box display="flex" justifyContent={"space-between"} width={"100%"} height={"100%"}>
        <Box width={"50%"} minWidth={0} overflow={"auto"}>
            <CommandResultTree commandResultProps={commandResultProps} onSelectNode={setSidePanelNode}
                               activeFilters={activeFilters}/>
        </Box>
        <Divider orientation="vertical" flexItem/>
        <Box width={"50%"} minWidth={0}>
            <SidePanel provider={sidePanelNode}/>
        </Box>
    </Box>
}

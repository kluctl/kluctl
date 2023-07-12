import * as React from 'react';
import { useContext, useState } from 'react';
import { Box, Drawer, ThemeProvider } from "@mui/material";
import { CommandResult, ShortName } from "../../models";
import { NodeData } from "./nodes/NodeData";
import { SidePanel } from "./SidePanel";
import CommandResultTree from "./CommandResultTree";
import { useParams } from "react-router-dom";
import { ApiContext, AppContext, useAppOutletContext } from "../App";
import { dark } from '../theme';
import { Api } from "../../api";
import { Loading, useLoadingHelper } from "../Loading";
import { CardPaper } from '../targets-view/Card';
import { ErrorMessage } from '../ErrorMessage';

export interface CommandResultProps {
    shortNames: ShortName[]
    commandResult: CommandResult
}

async function doLoadCommandResult(api: Api, resultId: string, shortNames: ShortName[]): Promise<CommandResultProps> {
    return {
        shortNames,
        commandResult: await api.getCommandResult(resultId),
    }
}

function DetailsDrawer(props: { nodeData?: NodeData, onClose?: () => void }) {
    return <ThemeProvider theme={dark}>
        <Drawer
            sx={{ zIndex: 1300 }}
            anchor={"right"}
            open={props.nodeData !== undefined}
            onClose={props.onClose}
            ModalProps={{ BackdropProps: { invisible: true } }}
        >
            <Box width={"720px"} height={"100%"}>
                <SidePanel provider={props.nodeData} onClose={props.onClose} />
            </Box>
        </Drawer>
    </ThemeProvider>;
}

export const CommandResultView = () => {
    const context = useAppOutletContext();
    const api = useContext(ApiContext);
    const appContext = useContext(AppContext);
    const [sidePanelNode, setSidePanelNode] = useState<NodeData | undefined>();

    const { id } = useParams()
    const [loading, loadingError, commandResultProps] = useLoadingHelper<CommandResultProps | undefined>(() => {
        if (!id) {
            return Promise.resolve(undefined)
        }
        return doLoadCommandResult(api, id, appContext.shortNames)
    }, [id])

    if (loading) {
        return <Loading />
    } else if (loadingError) {
        return <Box p='25px 40px' height='100%' overflow='hidden'>
            <CardPaper>
                <ErrorMessage>
                    {loadingError.message}
                </ErrorMessage>
            </CardPaper>
        </Box>
    }

    return <Box
        width='100%'
        height='100%'
        display='flex'
        flexDirection='column'
        overflow='hidden'
    >
        <DetailsDrawer
            nodeData={sidePanelNode}
            onClose={() => setSidePanelNode(undefined)}
        />
        <Box p='25px 40px' overflow='auto'>
            <CommandResultTree
                commandResultProps={commandResultProps}
                onSelectNode={setSidePanelNode}
                activeFilters={context.filters}
            />
        </Box>
    </Box>
}

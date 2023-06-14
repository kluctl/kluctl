import * as React from 'react';
import { useContext, useState } from 'react';
import {
    Box,
    Checkbox,
    CheckboxProps,
    Divider,
    Drawer,
    FormControlLabel,
    ThemeProvider,
    Typography
} from "@mui/material";
import { CommandResult, CommandResultSummary, ShortName } from "../../models";
import { NodeData } from "./nodes/NodeData";
import { SidePanel } from "./SidePanel";
import { ActiveFilters } from "./NodeStatusFilter";
import CommandResultTree from "./CommandResultTree";
import { useParams } from "react-router-dom";
import { ApiContext, useAppOutletContext } from "../App";
import { ChangesIcon, CheckboxCheckedIcon, CheckboxIcon, StarIcon, WarningSignIcon } from '../../icons/Icons';
import { dark } from '../theme';
import { Api } from "../../api";
import { Loading, useLoadingHelper } from "../Loading";

export interface CommandResultProps {
    shortNames: ShortName[]
    summary: CommandResultSummary
    commandResult: CommandResult
}

async function doLoadCommandResult(api: Api, resultId: string): Promise<CommandResultProps> {
    const shortNames = await api.getShortNames()
    const rs = await api.getResultSummary(resultId)
    const result = await api.getResult(resultId)

    return {
        shortNames: shortNames,
        summary: rs,
        commandResult: result,
    }
}

const FilterCheckbox = (props: {
    text: string,
    checked: boolean,
    Icon: () => JSX.Element,
    onChange: CheckboxProps['onChange']
}) => {
    const { text, checked, Icon, onChange } = props;
    return <FormControlLabel
        sx={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            margin: 0,
            padding: 0
        }}
        control={
            <Checkbox
                checked={checked}
                sx={{
                    display: 'flex',
                    justifyContent: 'center',
                    alignItems: 'center'
                }}
                onChange={onChange}
                icon={<CheckboxIcon />}
                checkedIcon={<CheckboxCheckedIcon />}
            />
        }
        slotProps={{
            typography: {
                sx: {
                    display: 'flex',
                    justifyContent: 'center',
                    alignItems: 'center',
                    gap: '10px',
                }
            }
        }}
        label={
            <>
                <Typography variant='h2'>{text}</Typography>
                <Box
                    flex='0 0 auto'
                    display='flex'
                    alignItems='center'
                    justifyContent='center'
                >
                    <Icon />
                </Box>
            </>
        }
    />
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

const defaultFilters = {
    onlyImportant: false,
    onlyChanged: false,
    onlyWithErrorsOrWarnings: false
}

export const CommandResultView = () => {
    const context = useAppOutletContext();
    const api = useContext(ApiContext)
    const [sidePanelNode, setSidePanelNode] = useState<NodeData | undefined>();

    const {id} = useParams()
    const [loading, loadingError, commandResultProps] = useLoadingHelper<CommandResultProps | undefined>(() => {
        if (!id) {
            return Promise.resolve(undefined)
        }
        return doLoadCommandResult(api, id)
    }, [id])

    const divider = <Divider
        orientation='vertical'
        sx={{
            height: '40px',
            margin: '0 20px 0 30px'
        }}
    />;

    const handleFilterChange = (filter: keyof ActiveFilters) => (_: React.ChangeEvent, checked: boolean) => {
        context.setFilters(fs => ({
            ...(fs || defaultFilters),
            [filter]: checked
        }));
    }

    if (loading) {
        return <Loading/>
    } else if (loadingError) {
        return <>Error</>
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
        <Box display='flex' alignItems='center' minHeight='70px' p='0 40px'>
            <FilterCheckbox
                text='Only important'
                checked={!!context.filters?.onlyImportant}
                Icon={StarIcon}
                onChange={handleFilterChange('onlyImportant')}
            />
            {divider}
            <FilterCheckbox
                text='Only with changes'
                checked={!!context.filters?.onlyChanged}
                Icon={ChangesIcon}
                onChange={handleFilterChange('onlyChanged')}
            />
            {divider}
            <FilterCheckbox
                text='Only with errors and warnings'
                checked={!!context.filters?.onlyWithErrorsOrWarnings}
                Icon={WarningSignIcon}
                onChange={handleFilterChange('onlyWithErrorsOrWarnings')}
            />
        </Box>
        <Divider sx={{ margin: '0 40px' }} />
        <Box p='25px 40px' overflow='auto'>
            <CommandResultTree
                commandResultProps={commandResultProps}
                onSelectNode={setSidePanelNode}
                activeFilters={context.filters}
            />
        </Box>
    </Box>
}

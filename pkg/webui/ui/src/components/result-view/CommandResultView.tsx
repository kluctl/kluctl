import * as React from 'react';
import { useState } from 'react';
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
import { useLoaderData } from "react-router-dom";
import { useAppOutletContext } from "../App";
import { ChangesIcon, CheckboxCheckedIcon, CheckboxIcon, StarIcon, WarningSignIcon } from '../../icons/Icons';
import { dark } from '../theme';
import { api } from "../../api";

export interface CommandResultProps {
    shortNames: ShortName[]
    summary: CommandResultSummary
    commandResult: CommandResult
}

export async function commandResultLoader({ params }: any) {
    const result = api.getResult(params.id)
    const shortNames = api.getShortNames()
    const rs = api.getResult(params.id)

    return {
        shortNames: await shortNames,
        summary: await rs,
        commandResult: await result,
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
    const commandResultProps = useLoaderData() as CommandResultProps;
    const [sidePanelNode, setSidePanelNode] = useState<NodeData | undefined>();

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

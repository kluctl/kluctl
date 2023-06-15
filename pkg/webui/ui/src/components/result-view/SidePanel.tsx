import { Box, Divider, IconButton, Paper, Tab, ThemeProvider, Typography, useTheme } from "@mui/material";
import React, { useCallback, useEffect, useMemo, useState } from "react";
import { TabContext, TabList, TabPanel } from "@mui/lab";
import { CloseIcon } from "../../icons/Icons";
import { light } from "../theme";

export interface SidePanelTab {
    label: string
    content: React.ReactNode
}

export interface SidePanelProvider {
    buildSidePanelTitle(): React.ReactNode
    buildSidePanelTabs(): SidePanelTab[]
}

export interface SidePanelProps {
    provider?: SidePanelProvider;
    onClose?: () => void;
}

export function useSidePanelTabs(provider?: SidePanelProvider) {
    const [selectedTab, setSelectedTab] = useState<string>();

    const handleTabChange = useCallback((_e: React.SyntheticEvent, value: string) => {
        setSelectedTab(value);
    }, []);

    const tabs = useMemo(
        () => provider?.buildSidePanelTabs() || [],
        [provider]
    );

    useEffect(() => {
        if (!tabs?.length) {
            setSelectedTab("");
            return
        }

        if (!selectedTab) {
            setSelectedTab(tabs[0].label);
            return
        }

        if (!tabs.find(x => x.label === selectedTab)) {
            // reset it after the type of selected item has changed
            setSelectedTab(tabs[0].label);
        }
    }, [selectedTab, tabs]);

    return { tabs, selectedTab, handleTabChange }
}

export const SidePanel = (props: SidePanelProps) => {
    const theme = useTheme();
    const { tabs, selectedTab, handleTabChange } = useSidePanelTabs(props.provider)

    if (!selectedTab || !tabs.find(x => x.label === selectedTab)) {
        return <></>
    }

    if (!props.provider) {
        return <></>
    }

    return <Box width={"100%"} height={"100%"} display="flex" flexDirection="column" overflow='hidden'>
        <TabContext value={selectedTab}>
            <Box minHeight={theme.consts.appBarHeight} display='flex' flexDirection='column' flex='0 0 auto' justifyContent='space-between'>
                <Box flex='1 1 auto' display='flex' justifyContent='space-between'>
                    <Box flex='1 1 auto' pt='25px' pl='35px'>
                        <Typography variant="h4">
                            {props.provider.buildSidePanelTitle()}
                        </Typography>
                    </Box>
                    <Box flex='0 0 auto' pt='10px' pr='10px'>
                        <IconButton onClick={props.onClose}>
                            <CloseIcon />
                        </IconButton>
                    </Box>
                </Box>
                <Box height='36px' flex='0 0 auto' p='0 30px'>
                    <TabList onChange={handleTabChange}>
                        {tabs.map((tab, i) => {
                            return <Tab label={tab.label} value={tab.label} key={tab.label} />
                        })}
                    </TabList>
                </Box>
            </Box>
            <Divider sx={{ margin: 0 }} />
            <Box overflow='auto' p='30px'>
                {tabs.map(tab => {
                    return <ThemeProvider theme={light} key={tab.label}>
                        <TabPanel value={tab.label} sx={{ padding: 0 }}>
                            <Paper sx={{ padding: '10px 0' }}>
                                {tab.content}
                            </Paper>
                        </TabPanel>
                    </ThemeProvider>
                })}
            </Box>
        </TabContext>
    </Box>
}

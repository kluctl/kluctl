import { Box, Divider, Tab, ThemeProvider } from "@mui/material";
import React, { useCallback, useEffect, useMemo, useState } from "react";
import { TabContext, TabList, TabPanel } from "@mui/lab";
import { light } from "../theme";
import { User } from "../../api";
import { useAppContext } from "../App";

export interface SidePanelTab {
    label: string
    content: React.ReactNode
}

export interface SidePanelProvider {
    buildSidePanelTitle(): React.ReactNode
    buildSidePanelTabs(user?: User): SidePanelTab[]
}

export interface SidePanelProps {
    provider?: SidePanelProvider;
    onClose?: () => void;
}

export function useSidePanelTabs(provider?: SidePanelProvider) {
    const [selectedTab, setSelectedTab] = useState<string>();
    const appCtx = useAppContext()

    const handleTabChange = useCallback((_e: React.SyntheticEvent, value: string) => {
        setSelectedTab(value);
    }, []);

    const tabs = useMemo(
        () => provider?.buildSidePanelTabs(appCtx.user) || [],
        [provider, appCtx.user]
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
    const { tabs, selectedTab, handleTabChange } = useSidePanelTabs(props.provider)

    if (!selectedTab || !tabs.find(x => x.label === selectedTab)) {
        return <></>
    }

    if (!props.provider) {
        return <></>
    }

    return <Box width={"100%"} height={"100%"} display="flex" flexDirection="column" overflow='hidden'>
        <TabContext value={selectedTab}>
            <Box display='flex' flexDirection='column' flex='0 0 auto' justifyContent='space-between'>
                <Box height='36px' flex='0 0 auto' p='0 30px'>
                    <TabList onChange={handleTabChange}>
                        {tabs.map((tab, i) => {
                            return <Tab label={tab.label} value={tab.label} key={tab.label} />
                        })}
                    </TabList>
                </Box>
            </Box>
            <Divider />
            <Box overflow='auto' p='4px' flex={"1 1 auto"}>
                {tabs.map(tab => {
                    return <ThemeProvider theme={light} key={tab.label}>
                        <TabPanel value={tab.label} sx={{padding: 0, display: "flex"}}>
                            {tab.content}
                        </TabPanel>
                    </ThemeProvider>
                })}
            </Box>
        </TabContext>
    </Box>
}

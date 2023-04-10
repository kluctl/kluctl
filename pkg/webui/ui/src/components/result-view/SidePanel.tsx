import { Box, Tab, Typography } from "@mui/material";
import React, { useEffect, useState } from "react";
import { TabContext, TabList, TabPanel } from "@mui/lab";

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
}

export const SidePanel = (props: SidePanelProps) => {
    let [selectedTab, setSelectedTab] = useState<string>();

    function handleTabChange(_e: React.SyntheticEvent, value: string) {
        setSelectedTab(value);
    }

    let tabs = props.provider?.buildSidePanelTabs()
    if (!tabs) {
        tabs = []
    }

    useEffect(() => {
        if (!tabs?.length) {
            setSelectedTab("")
            return
        }

        if (!selectedTab) {
            setSelectedTab(tabs[0].label)
            return
        }

        if (!tabs.find(x => x.label === selectedTab)) {
            // reset it after the type of selected item has changed
            setSelectedTab(tabs[0].label)
        }
        // ignore that it wants us to add selectedTab to the deps (it would cause and endless loop)
        // eslint-disable-next-line
    }, [props.provider])

    if (!selectedTab || !tabs.find(x => x.label === selectedTab)) {
        return <></>
    }

    if (!props.provider) {
        return <></>
    }

    return <Box width={"100%"} height={"100%"} display="flex" flexDirection="column" p={3}>
        <Typography variant="h4" mb={1} component="div">
            {props.provider.buildSidePanelTitle()}
        </Typography>

        <TabContext value={selectedTab}>
            <Box flex="0 0 auto" sx={{ borderBottom: 1, borderColor: 'divider' }}>
                <TabList onChange={handleTabChange}>
                    {tabs.map((tab, i) => {
                        return <Tab label={tab.label} value={tab.label} key={tab.label}/>
                    })}
                </TabList>
            </Box>
            {tabs.map((tab, index) => {
                return <TabPanel value={tab.label} key={index} sx={{ overflowY: "auto" }}>
                    {tab.content}
                </TabPanel>
            })}
        </TabContext>
    </Box>
}

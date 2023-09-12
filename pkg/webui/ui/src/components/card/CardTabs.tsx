import { Box, Divider, Tab } from "@mui/material";
import React, { useCallback, useEffect, useMemo, useState } from "react";
import { TabContext, TabList, TabPanel } from "@mui/lab";
import { AppContextProps, useAppContext } from "../App";

export interface CardTab {
    label: string
    content: React.ReactNode
}

export interface CardTabsProvider {
    buildSidePanelTitle(): React.ReactNode
    buildSidePanelTabs(appContext: AppContextProps): CardTab[]
}

export interface CardTabsProps {
    provider?: CardTabsProvider;
    onClose?: () => void;
}

export function useCardTabs(provider?: CardTabsProvider) {
    const [selectedTab, setSelectedTab] = useState<string>();
    const appCtx = useAppContext()

    const handleTabChange = useCallback((_e: React.SyntheticEvent, value: string) => {
        setSelectedTab(value);
    }, []);

    const tabs = useMemo(
        () => provider?.buildSidePanelTabs(appCtx) || [],
        [provider, appCtx]
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

export const CardTabs = (props: CardTabsProps) => {
    const { tabs, selectedTab, handleTabChange } = useCardTabs(props.provider)

    if (!selectedTab || !tabs.find(x => x.label === selectedTab)) {
        return <></>
    }

    if (!props.provider) {
        return <></>
    }


    return <TabContext value={selectedTab}>
        <Box display='flex' flexDirection='column' height='100%' overflow='hidden'>
            <Box height='36px' flex='0 0 auto' p='0'>
                <TabList onChange={handleTabChange}>
                    {tabs.map((tab, i) => {
                        return <Tab id={tab.label} label={tab.label} value={tab.label} key={tab.label}/>
                    })}
                </TabList>
            </Box>
            <Divider sx={{ margin: 0 }}/>
            <Box overflow='auto' p='10px 0' display={"flex"} flex={"1 1 auto"}>
                {tabs.map(tab => {
                    const sx: any = { padding: 0, flex: "1 1 auto" }
                    if (selectedTab === tab.label) {
                        // only the active tab should be a flex box, as otherwise the hidden ones go crazy
                        sx.display = "flex"
                    }
                    return <TabPanel key={tab.label} value={tab.label} sx={sx}>
                        {tab.content}
                    </TabPanel>
                })}
            </Box>
        </Box>
    </TabContext>
}

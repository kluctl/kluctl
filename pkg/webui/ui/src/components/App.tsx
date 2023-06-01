import React, { Dispatch, SetStateAction, useState } from 'react';

import '../index.css';
import { Box, ThemeProvider } from "@mui/material";
import { Outlet, useOutletContext } from "react-router-dom";
import LeftDrawer from "./LeftDrawer";
import { light } from './theme';
import { ActiveFilters } from './result-view/NodeStatusFilter';

export interface AppOutletContext {
    filters?: ActiveFilters
    setFilters: Dispatch<SetStateAction<ActiveFilters | undefined>>
}

export function useAppOutletContext(): AppOutletContext {
    return useOutletContext<AppOutletContext>()
}

const App = () => {
    const [filters, setFilters] = useState<ActiveFilters>()

    const context: AppOutletContext = {
        filters: filters,
        setFilters: setFilters,
    }

    return (
        <ThemeProvider theme={light}>
            <Box width={"100%"} height={"100%"}>
                <LeftDrawer content={<Outlet context={context} />} context={context} />
            </Box>
        </ThemeProvider>
    );
};

export default App;

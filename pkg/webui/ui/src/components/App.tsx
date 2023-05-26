import React, { useState } from 'react';

import '../index.css';
import { Box, ThemeProvider } from "@mui/material";
import { Outlet, useOutletContext } from "react-router-dom";
import LeftDrawer from "./LeftDrawer";
import { light } from './theme';

export interface AppOutletContext {
    filters?: React.ReactNode
    setFilters: (filter: React.ReactNode) => void
}

export function useAppOutletContext(): AppOutletContext {
    return useOutletContext<AppOutletContext>()
}

const App = () => {
    const [filters, setFilters] = useState<React.ReactNode>()

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

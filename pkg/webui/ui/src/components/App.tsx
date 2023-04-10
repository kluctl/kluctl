import React, { useState } from 'react';

import '../index.css';
import { Box } from "@mui/material";
import { Outlet, useOutletContext } from "react-router-dom";
import LeftDrawer from "./LeftDrawer";

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

    return <Box width={"100%"} height={"100%"}>
        <LeftDrawer content={<Outlet context={context}/>} context={context}/>
    </Box>
};

export default App;

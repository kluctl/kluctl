import React, { createContext, Dispatch, SetStateAction, useCallback, useEffect, useMemo, useState } from 'react';

import '../index.css';
import { Box, ThemeProvider } from "@mui/material";
import { Outlet, useOutletContext } from "react-router-dom";
import LeftDrawer from "./LeftDrawer";
import { light } from './theme';
import { ActiveFilters } from './result-view/NodeStatusFilter';
import { CommandResultSummary, ProjectTargetKey, ValidateResult } from "../models";
import { api } from "../api";
import { buildProjectSummaries, ProjectSummary } from "../project-summaries";

export interface AppOutletContext {
    filters?: ActiveFilters
    setFilters: Dispatch<SetStateAction<ActiveFilters | undefined>>
}
export function useAppOutletContext(): AppOutletContext {
    return useOutletContext<AppOutletContext>()
}

export interface AppContextProps {
    summaries: Map<string, CommandResultSummary>
    projects: ProjectSummary[]
    validateResults: Map<string, ValidateResult>
}
export const AppContext = createContext<AppContextProps>({
    summaries: new Map(),
    projects: [],
    validateResults: new Map(),
});

const App = () => {
    let [summaries, setSummaries] = useState<Map<string, CommandResultSummary>>(new Map())
    let [validateResults, setValidateResults] = useState<Map<string, ValidateResult>>(new Map())
    const [filters, setFilters] = useState<ActiveFilters>()

    const updateSummary = (rs: CommandResultSummary) => {
        console.log("update_summary", rs.id, rs.commandInfo.startTime)
        summaries.set(rs.id, rs)
        summaries = new Map(summaries)
        setSummaries(summaries)
    }

    const deleteSummary = (id: string) => {
        console.log("delete_summary", id)
        summaries.delete(id)
        summaries = new Map(summaries)
        setSummaries(summaries)
    }

    const updateValidateResult = (key: ProjectTargetKey, vr: ValidateResult) => {
        console.log("validate_result", key)
        validateResults.set(JSON.stringify(key), vr)
        validateResults = new Map(validateResults)
        setValidateResults(validateResults)
    }

    useEffect(() => {
        console.log("starting listenResults")
        const cancel = api.listenUpdates(undefined, undefined, msg => {
            switch(msg.type) {
                case "update_summary":
                    updateSummary(msg.summary)
                    break
                case "delete_summary":
                    deleteSummary(msg.id)
                    break
                case "validate_result":
                    updateValidateResult(msg.key, msg.result)
                    break
            }
        })
        return () => {
            console.log("cancel listenResults")
            cancel.then(c => c())
        }
    }, [])

    const projects = useMemo(() => {
        return buildProjectSummaries(summaries, validateResults)
    }, [summaries, validateResults])

    const appContext = {
        summaries: summaries,
        projects: projects,
        validateResults: validateResults,
    }

    const outletContext: AppOutletContext = {
        filters: filters,
        setFilters: setFilters,
    }

    return (
        <AppContext.Provider value={appContext}>
            <ThemeProvider theme={light}>
                <Box width={"100%"} height={"100%"}>
                    <LeftDrawer content={<Outlet context={outletContext}/>} context={outletContext}/>
                </Box>
            </ThemeProvider>
        </AppContext.Provider>
    );
};

export default App;

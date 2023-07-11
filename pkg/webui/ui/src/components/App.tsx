import { createContext, Dispatch, SetStateAction, useContext, useEffect, useMemo, useRef, useState } from 'react';

import '../index.css';
import { Box } from "@mui/material";
import { Outlet, useOutletContext } from "react-router-dom";
import LeftDrawer from "./LeftDrawer";
import { ActiveFilters } from './result-view/NodeStatusFilter';
import { CommandResultSummary, ShortName, ValidateResultSummary } from "../models";
import { Api, checkStaticBuild, RealApi, StaticApi } from "../api";
import { buildProjectSummaries, ProjectSummary } from "../project-summaries";
import Login from "./Login";
import { Loading, useLoadingHelper } from "./Loading";
import { ErrorMessageCard } from './ErrorMessage';

export interface AppOutletContext {
    filters?: ActiveFilters
    setFilters: Dispatch<SetStateAction<ActiveFilters | undefined>>
}
export function useAppOutletContext(): AppOutletContext {
    return useOutletContext<AppOutletContext>()
}

export interface AppContextProps {
    commandResultSummaries: Map<string, CommandResultSummary>
    projects: ProjectSummary[]
    validateResultSummaries: Map<string, ValidateResultSummary>
    shortNames: ShortName[]
}
export const AppContext = createContext<AppContextProps>({
    commandResultSummaries: new Map(),
    projects: [],
    validateResultSummaries: new Map(),
    shortNames: []
});

export const ApiContext = createContext<Api>(new StaticApi())

export interface KluctlDeploymentWithClusterId {
    deployment: any
    clusterId: string
}

const LoggedInApp = (props: { onUnauthorized: () => void }) => {
    const api = useContext(ApiContext)
    const [filters, setFilters] = useState<ActiveFilters>()

    const commandResultSummariesRef = useRef<Map<string, CommandResultSummary>>(new Map())
    const validateResultSummariesRef = useRef<Map<string, ValidateResultSummary>>(new Map())
    const kluctlDeploymentsRef = useRef<Map<string, KluctlDeploymentWithClusterId>>(new Map())
    const [commandResultSummaries, setCommandResultSummaries] = useState(commandResultSummariesRef.current)
    const [validateResultSummaries, setValidateResultSummaries] = useState(validateResultSummariesRef.current)
    const [kluctlDeployments, setKluctlDeployments] = useState(kluctlDeploymentsRef.current)

    useEffect(() => {
        const updateCommandResultSummary = (rs: CommandResultSummary) => {
            console.log("update_command_result_summary", rs.id, rs.commandInfo.startTime)
            commandResultSummariesRef.current.set(rs.id, rs)
            setCommandResultSummaries(new Map(commandResultSummariesRef.current))
        }

        const deleteCommandResultSummary = (id: string) => {
            console.log("delete_command_result_summary", id)
            commandResultSummariesRef.current.delete(id)
            setCommandResultSummaries(new Map(commandResultSummariesRef.current))
        }

        const updateValidateResultSummary = (vr: ValidateResultSummary) => {
            console.log("update_validate_result_summary", vr.id)
            validateResultSummariesRef.current.set(vr.id, vr)
            setValidateResultSummaries(new Map(validateResultSummariesRef.current))
        }

        const deleteValidateResultSummary = (id: string) => {
            console.log("delete_validate_result_summary", id)
            validateResultSummariesRef.current.delete(id)
            setValidateResultSummaries(new Map(validateResultSummariesRef.current))
        }

        const updateKluctlDeployment = (kd: any, clusterId: string) => {
            console.log("update_kluctl_deployment", kd.metadata.uid, kd.metadata.name)
            kluctlDeploymentsRef.current.set(kd.metadata.uid, {
                deployment: kd,
                clusterId: clusterId,
            })
            setKluctlDeployments(new Map(kluctlDeploymentsRef.current))
        }

        const deleteKluctlDeployment = (id: string) => {
            console.log("delete_kluctl_deployment", id)
            kluctlDeploymentsRef.current.delete(id)
            setKluctlDeployments(new Map(kluctlDeploymentsRef.current))
        }

        console.log("starting listenResults")
        let cancel: Promise<() => void>
        cancel = api.listenUpdates(undefined, undefined, msg => {
            switch (msg.type) {
                case "update_command_result_summary":
                    updateCommandResultSummary(msg.summary)
                    break
                case "delete_command_result_summary":
                    deleteCommandResultSummary(msg.id)
                    break
                case "update_validate_result_summary":
                    updateValidateResultSummary(msg.summary)
                    break
                case "delete_validate_result_summary":
                    deleteValidateResultSummary(msg.id)
                    break
                case "update_kluctl_deployment":
                    updateKluctlDeployment(msg.deployment, msg.clusterId)
                    break
                case "delete_kluctl_deployment":
                    deleteKluctlDeployment(msg.id)
                    break
            }
        })
        return () => {
            console.log("cancel listenResults")
            cancel.then(c => c())
        }
    }, [api])

    const projects = useMemo(() => {
        return buildProjectSummaries(commandResultSummaries, validateResultSummaries, kluctlDeployments, false)
    }, [commandResultSummaries, validateResultSummaries, kluctlDeployments])

    const [loading, loadingError, shortNames] = useLoadingHelper<ShortName[]>(
        () => api.getShortNames(),
        [api]
    );
    
    if (loading) {
        return <Loading />;
    }

    if (loadingError) {
        return <ErrorMessageCard>
            {loadingError.message}
        </ErrorMessageCard>;
    }

    const appContext: AppContextProps = {
        commandResultSummaries: commandResultSummariesRef.current,
        projects,
        validateResultSummaries: validateResultSummaries,
        shortNames: shortNames || []
    }

    const outletContext: AppOutletContext = {
        filters: filters,
        setFilters: setFilters,
    }

    return (
        <AppContext.Provider value={appContext}>
            <Box width={"100%"} height={"100%"}>
                <LeftDrawer
                    content={<Outlet context={outletContext} />}
                    context={outletContext}
                    logout={props.onUnauthorized}
                />
            </Box>
        </AppContext.Provider>
    );
};

const App = () => {
    const [api, setApi] = useState<Api>()
    const [needToken, setNeedToken] = useState(false)

    const storage = localStorage

    const getToken = () => {
        const token = storage.getItem("token")
        if (!token) {
            return ""
        }
        return JSON.parse(token)
    }
    const setToken = (token?: string) => {
        if (!token) {
            storage.removeItem("token")
        } else {
            storage.setItem("token", JSON.stringify(token))
        }
    }

    const onUnauthorized = () => {
        console.log("handle onUnauthorized")
        setToken(undefined)
        setApi(undefined)
        setNeedToken(true)
    }
    const onTokenRefresh = (newToken: string) => {
        console.log("handle onTokenRefresh")
        setToken(newToken)
    }

    const handleLoginSucceeded = (token: string) => {
        console.log("handle saveToken")
        setToken(token);
        setApi(new RealApi(getToken, onUnauthorized, onTokenRefresh))
    };

    useEffect(() => {
        if (api) {
            return
        }

        const doInit = async () => {
            const isStatic = await checkStaticBuild()
            if (isStatic) {
                setApi(new StaticApi())
            } else {
                // check if we don't need auth (running locally?)
                const noAuthApi = new RealApi(undefined, undefined, undefined)
                try {
                    await noAuthApi.getShortNames()
                    setToken(undefined)
                    setNeedToken(false)
                    setApi(noAuthApi)
                } catch (error) {
                    if (!getToken()) {
                        setNeedToken(true)
                    } else {
                        setApi(new RealApi(getToken, onUnauthorized, onTokenRefresh))
                    }
                }
            }
        }
        doInit()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    if (needToken && !getToken()) {
        return <Login setToken={handleLoginSucceeded} />
    }

    if (!api) {
        return <Loading />
    }

    return <ApiContext.Provider value={api}>
        <LoggedInApp onUnauthorized={onUnauthorized} />
    </ApiContext.Provider>
}

export default App;

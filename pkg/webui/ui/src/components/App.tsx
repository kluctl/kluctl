import {
    createContext,
    Dispatch,
    SetStateAction,
    useContext,
    useEffect,
    useMemo,
    useRef,
    useState
} from 'react';

import '../index.css';
import { Box } from "@mui/material";
import { Outlet, useOutletContext } from "react-router-dom";
import LeftDrawer from "./LeftDrawer";
import { ActiveFilters } from './result-view/NodeStatusFilter';
import { CommandResultSummary, ProjectTargetKey, ShortName, ValidateResult } from "../models";
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
    summaries: Map<string, CommandResultSummary>
    projects: ProjectSummary[]
    validateResults: Map<string, ValidateResult>
    shortNames: ShortName[]
}
export const AppContext = createContext<AppContextProps>({
    summaries: new Map(),
    projects: [],
    validateResults: new Map(),
    shortNames: []
});

export const ApiContext = createContext<Api>(new StaticApi())

const LoggedInApp = (props: { onUnauthorized: () => void }) => {
    const api = useContext(ApiContext)
    const [filters, setFilters] = useState<ActiveFilters>()

    const summariesRef = useRef<Map<string, CommandResultSummary>>(new Map())
    const validateResultsRef = useRef<Map<string, ValidateResult>>(new Map())
    const [summaries, setSummaries] = useState(summariesRef.current)
    const [validateResults, setValidateResults] = useState(validateResultsRef.current)

    const onUnauthorized = props.onUnauthorized

    useEffect(() => {
        const updateSummary = (rs: CommandResultSummary) => {
            console.log("update_summary", rs.id, rs.commandInfo.startTime)
            summariesRef.current.set(rs.id, rs)
            setSummaries(new Map(summariesRef.current))
        }

        const deleteSummary = (id: string) => {
            console.log("delete_summary", id)
            summariesRef.current.delete(id)
            setSummaries(new Map(summariesRef.current))
        }

        const updateValidateResult = (key: ProjectTargetKey, vr: ValidateResult) => {
            console.log("validate_result", key)
            validateResultsRef.current.set(JSON.stringify(key), vr)
            setValidateResults(new Map(validateResultsRef.current))
        }

        console.log("starting listenResults")
        let cancel: Promise<() => void>
        cancel = api.listenUpdates(undefined, undefined, msg => {
            switch (msg.type) {
                case "update_summary":
                    updateSummary(msg.summary)
                    break
                case "delete_summary":
                    deleteSummary(msg.id)
                    break
                case "validate_result":
                    updateValidateResult(msg.key, msg.result)
                    break
                case "auth_result":
                    if (!msg.success) {
                        cancel.then(c => c())
                        onUnauthorized()
                    }
            }
        })
        return () => {
            console.log("cancel listenResults")
            cancel.then(c => c())
        }
    }, [api, onUnauthorized])

    const projects = useMemo(() => {
        return buildProjectSummaries(summaries, validateResults)
    }, [summaries, validateResults])

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
        summaries: summariesRef.current,
        projects,
        validateResults,
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
                    logout={onUnauthorized}
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

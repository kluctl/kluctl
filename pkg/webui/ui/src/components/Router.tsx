import { createHashRouter, useRouteError, useSearchParams } from "react-router-dom";
import App, { AppContextProps, useAppContext } from "./App";
import { ErrorMessageCard } from "./ErrorMessage";
import { Box } from "@mui/material";
import React, { useEffect, useState } from "react";
import { TargetsView } from "./target-view/TargetsView";
import { CommandResultSummary } from "../models";
import { buildTargetPath, ProjectSummary, TargetSummary } from "../project-summaries";
import { Loading } from "./Loading";

function ErrorPage() {
    const error = useRouteError() as any;

    return <ErrorMessageCard>
        <Box>{error.statusText}</Box>
        <Box>{error.data}</Box>
    </ErrorMessageCard>
}

function buildRedirectToTargetPath(app: AppContextProps, sp: URLSearchParams) {
    const gitRepoKey = sp.get("gitRepoKey")
    const subDir = sp.get("subDir")
    const targetName = sp.get("targetName")
    const targetClusterId = sp.get("targetClusterId")
    const discriminator = sp.get("discriminator")
    const commandResultId = sp.get("commandResultId")
    const kdName = sp.get("kluctlDeploymentName")
    const kdNamespace = sp.get("kluctlDeploymentNamespace")
    const kdClusterId = sp.get("kluctlDeploymentClusterId")

    if (!gitRepoKey && !subDir && !targetName && !targetClusterId && !discriminator &&
        !kdName && !kdNamespace && !kdClusterId &&
        !commandResultId) {
        return "/targets"
    }

    let firstProject: ProjectSummary | undefined
    let firstTarget: TargetSummary | undefined

    let firstCommandResultSummary: CommandResultSummary | undefined
    if (commandResultId) {
        app.projects.forEach(ps => {
            ps.targets.forEach(ts => {
                ts.commandResults.forEach(rs => {
                    if (rs.id === commandResultId && !firstCommandResultSummary) {
                        firstProject = ps
                        firstTarget = ts
                        firstCommandResultSummary = rs
                    }
                })
            })
        })
    } else {
        app.projects.forEach(ps => {
            if (firstProject) return
            if (gitRepoKey !== null && gitRepoKey !== ps.project.gitRepoKey) return
            if (subDir !== null && subDir !== ps.project.subDir) return

            ps.targets.forEach(ts => {
                if (firstTarget) return
                if (targetName !== null && targetName !== ts.target.targetName) return
                if (targetClusterId !== null && targetClusterId !== ts.target.clusterId) return
                if (discriminator !== null && discriminator !== ts.target.discriminator) return
                if (kdName !== null && kdName !== ts.kdInfo?.name) return
                if (kdNamespace !== null && kdNamespace !== ts.kdInfo?.namespace) return
                if (kdClusterId !== null && kdClusterId !== ts.kdInfo?.clusterId) return

                firstProject = ps
                firstTarget = ts
            })
        })
    }

    let redirectPath: string | undefined = undefined
    if (firstProject && firstTarget) {
        redirectPath = buildTargetPath(firstProject, firstTarget, !!firstCommandResultSummary, firstCommandResultSummary, false)
    }

    return redirectPath
}

function RedirectToTarget(props: {}) {
    const [spFromRouter] = useSearchParams()
    const spFromWindow = new URLSearchParams(window.location.search)

    // we need to merge the search params from the window.location and from the react-router location
    const sp = new URLSearchParams(spFromWindow)
    spFromRouter.forEach((k, v) => {
        sp.set(k, v)
    })

    const app = useAppContext()
    const [timedout, setTimedout] = useState(false)

    const redirectPath = buildRedirectToTargetPath(app, sp)

    useEffect(() => {
        console.log("start timer")
        const timer = setTimeout(() => {
            console.log("setTimedout")
            setTimedout(true)
        }, 5000)
        return () => {
            console.log("stop timer")
            clearTimeout(timer)
        }
    }, [])

    useEffect(() => {
        if (timedout) {
            console.log("timed out waiting for results/projects")
            window.location.replace("/#/targets")
            return
        }
        if (!redirectPath) {
            return
        }

        window.location.replace("/#" + redirectPath)
    }, [redirectPath, timedout]);

    return <Loading/>
}

export const Router = createHashRouter([
    {
        path: "/",
        element: <App />,
        errorElement: <ErrorPage />,
        children: [
            {
                path: "/",
                element: <RedirectToTarget/>,
                errorElement: <ErrorPage />,
            },
            {
                path: "targets/*",
                element: <TargetsView />,
                errorElement: <ErrorPage />,
            },
        ],
    },
]);

import { createBrowserRouter, createHashRouter, useRouteError } from "react-router-dom";
import React from "react";
import App from "./App";
import { projectsLoader, TargetsView } from "./targets-view/TargetsView";
import { commandResultLoader, CommandResultView } from "./result-view/CommandResultView";

function ErrorPage() {
    const error = useRouteError() as any;

    return (
        <div id="error-page">
            <h1>Oops!</h1>
            <p>Sorry, an unexpected error has occurred.</p>
            <p>
                <i>{error.statusText || error.message}</i>
            </p>
        </div>
    );
}

export const Router = createHashRouter([
    {
        path: "/",
        element: <App />,
        errorElement: <ErrorPage />,
        children: [
            {
                path: "targets",
                element: <TargetsView/>,
                loader: projectsLoader,
                errorElement: <ErrorPage/>,
            },
            {
                path: "results/:id",
                element: <CommandResultView/>,
                loader: commandResultLoader,
                errorElement: <ErrorPage/>,
            },
        ],
    },
]);

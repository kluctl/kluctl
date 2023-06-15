import { createHashRouter, useRouteError } from "react-router-dom";
import React from "react";
import App from "./App";
import { TargetsView } from "./targets-view/TargetsView";
import { CommandResultView } from "./result-view/CommandResultView";

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
                path: "targets/:targetKeyHash?",
                element: <TargetsView/>,
                errorElement: <ErrorPage/>,
            },
            {
                path: "results/:id",
                element: <CommandResultView/>,
                errorElement: <ErrorPage/>,
            },
        ],
    },
]);

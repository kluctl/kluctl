import { createHashRouter, useRouteError } from "react-router-dom";
import React from "react";
import App from "./App";
import { TargetsView } from "./targets-view/TargetsView";
import { CommandResultView } from "./result-view/CommandResultView";
import { ErrorMessage } from "./ErrorMessage";
import { CardPaper } from "./targets-view/Card";
import { Box } from "@mui/material";

function ErrorPage() {
    const error = useRouteError() as any;

    return <Box
        width='100%'
        height='100%'
        overflow='hidden'
        p='40px'
    >
        <CardPaper>
            <ErrorMessage>
                <Box>{error.statusText}</Box>
                <Box>{error.data}</Box>
            </ErrorMessage>
        </CardPaper>
    </Box>
}

export const Router = createHashRouter([
    {
        path: "/",
        element: <App />,
        errorElement: <ErrorPage />,
        children: [
            {
                path: "targets/:targetKeyHash?",
                element: <TargetsView />,
                errorElement: <ErrorPage />,
            },
            {
                path: "results/:id",
                element: <CommandResultView />,
                errorElement: <ErrorPage />,
            },
        ],
    },
]);

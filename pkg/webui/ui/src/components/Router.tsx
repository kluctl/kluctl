import { createHashRouter, useNavigate, useRouteError } from "react-router-dom";
import App from "./App";
import { ErrorMessageCard } from "./ErrorMessage";
import { Box } from "@mui/material";
import React, { useEffect } from "react";
import { TargetsView } from "./target-view/TargetsView";

function ErrorPage() {
    const error = useRouteError() as any;

    return <ErrorMessageCard>
        <Box>{error.statusText}</Box>
        <Box>{error.data}</Box>
    </ErrorMessageCard>
}

function Redirect(props: { to: string }) {
    let navigate = useNavigate();
    useEffect(() => {
        navigate(props.to, {replace: true});
    });
    return null;
}

export const Router = createHashRouter([
    {
        path: "/",
        element: <App />,
        errorElement: <ErrorPage />,
        children: [
            {
                path: "/",
                element: <Redirect to={"/targets"}/>,
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

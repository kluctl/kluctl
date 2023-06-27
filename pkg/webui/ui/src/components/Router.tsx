import { createHashRouter, useRouteError } from "react-router-dom";
import App from "./App";
import { TargetsView } from "./targets-view/TargetsView";
import { CommandResultView } from "./result-view/CommandResultView";
import { ErrorMessageCard } from "./ErrorMessage";
import { Box } from "@mui/material";

function ErrorPage() {
    const error = useRouteError() as any;

    return <ErrorMessageCard>
        <Box>{error.statusText}</Box>
        <Box>{error.data}</Box>
    </ErrorMessageCard>
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

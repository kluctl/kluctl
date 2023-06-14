import { Box, CircularProgress } from "@mui/material";
import { DependencyList, useEffect, useState } from "react";

export const Loading = () => {
    return <Box display={"flex"} height={"100%"}>
        <Box
            display={"flex"}
            width={"100%"}
            height={"100%"}
            alignItems={"center"}
            justifyContent={"center"}
        >
            <CircularProgress/>
        </Box>
    </Box>
}

export function useLoadingHelper<T>(load: () => Promise<T>, deps: DependencyList): [boolean, any, T | undefined] {
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<any>()
    const [content, setContent] = useState<T>()

    useEffect(() => {
        const doStartLoading = async () => {
            try {
                const c = await load()
                setContent(c)
                setLoading(false)
            } catch (error) {
                setError(error)
                setLoading(false)
            }
        }
        doStartLoading()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, deps)

    return [loading, error, content]
}

import { Box, CircularProgress } from "@mui/material";

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
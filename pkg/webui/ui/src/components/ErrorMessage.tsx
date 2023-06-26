import { Box, Typography, useTheme } from "@mui/material";

export interface ErrorMessageProps {
    children?: React.ReactNode;
}

export const ErrorMessage = (props: ErrorMessageProps) => {
    const theme = useTheme();
    return <Box
        height='100%'
        width='100%'
        overflow='auto'
        p='30px'
        color={theme.palette.error.main}
        display='flex'
        flexDirection='column'
        gap='10px'
    >
        <Typography
            variant='h6'
            textAlign='left'
            color={theme.palette.error.main}
        >
            Error
        </Typography>
        {props.children}
    </Box>;
}

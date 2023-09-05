import { Box, Typography, useTheme } from "@mui/material";
import { CardPaper } from "./card/Card";

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

export const ErrorMessageCard = (props: ErrorMessageProps) => {
    return <Box
        width='100%'
        height='100%'
        overflow='hidden'
        p='40px'
    >
        <CardPaper>
            <ErrorMessage>
                {props.children}
            </ErrorMessage>
        </CardPaper>
    </Box>
}

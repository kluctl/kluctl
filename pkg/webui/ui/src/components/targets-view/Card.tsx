import React from "react";
import { Box, BoxProps, Paper, PaperProps } from "@mui/material"

export const cardWidth = 247;
export const projectCardHeight = 80;
export const cardHeight = 126;
export const cardGap = 20;

export function CardPaper(props: PaperProps) {
    const { sx, ...rest } = props;
    return <Paper
        elevation={5}
        sx={{
            width: "100%",
            height: "100%",
            borderRadius: '12px',
            border: '1px solid #59A588',
            boxShadow: '4px 4px 10px #1E617A',
            ...sx
        }}
        {...rest}
    />
}

export const Card = React.forwardRef((props: BoxProps, ref) => {
    return <Box display='flex' flexShrink={0} width={cardWidth} height={cardHeight} {...props} ref={ref} />
});

export function CardCol(props: BoxProps) {
    return <Box display='flex' flexDirection='column' gap={`${cardGap}px`} {...props} />
}

export function CardRow(props: BoxProps) {
    return <Box display='flex' gap={`${cardGap}px`} {...props} />
}

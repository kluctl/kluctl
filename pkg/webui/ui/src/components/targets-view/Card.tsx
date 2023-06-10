import { Box, BoxProps } from "@mui/material"

export const cardWidth = 247;
export const projectCardHeight = 80;
export const cardHeight = 126;
export const cardGap = 20;

export function Card({ children, ...rest }: BoxProps) {
    return <Box display='flex' flexShrink={0} width={cardWidth} height={cardHeight} {...rest}>
        {children}
    </Box>
}

export function CardCol({ children, ...rest }: BoxProps) {
    return <Box display='flex' flexDirection='column' gap={`${cardGap}px`} {...rest}>
        {children}
    </Box>
}

export function CardRow({ children, ...rest }: BoxProps) {
    return <Box display='flex' gap={`${cardGap}px`} {...rest}>
        {children}
    </Box>
}

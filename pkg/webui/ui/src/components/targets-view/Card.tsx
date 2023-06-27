import React from "react";
import { Box, BoxProps, Paper, PaperProps, Tooltip, Typography } from "@mui/material"

export const cardWidth = 247;
export const projectCardMinHeight = 80;
export const cardHeight = 126;
export const cardGap = 20;

export const CardPaper = React.forwardRef((props: PaperProps, ref: React.ForwardedRef<HTMLDivElement>) => {
    const { sx, ...rest } = props;
    return <Paper
        elevation={5}
        sx={{
            width: cardWidth,
            height: cardHeight,
            borderRadius: '12px',
            border: '1px solid #59A588',
            boxShadow: '4px 4px 10px #1E617A',
            flexShrink: 0,
            ...sx
        }}
        {...rest}
        ref={ref}
    />
});

CardPaper.displayName = 'CardPaper';

export function CardCol(props: BoxProps) {
    return <Box display='flex' flexDirection='column' gap={`${cardGap}px`} {...props} />
}

export function CardRow(props: BoxProps) {
    return <Box display='flex' gap={`${cardGap}px`} {...props} />
}

export const CardTemplate = React.forwardRef((props: {
    paperProps?: PaperProps,
    boxProps?: BoxProps,
    icon?: React.ReactNode,
    iconTooltip?: React.ReactNode,
    header?: string,
    headerTooltip?: React.ReactNode,
    subheader?: string,
    subheaderTooltip?: React.ReactNode,
    body?: React.ReactNode,
    footer?: React.ReactNode
}, ref: React.ForwardedRef<HTMLDivElement>) => {
    const icon = props.icon && (
        <Tooltip title={props.iconTooltip}>
            <Box
                width='45px'
                height='45px'
                flex='0 0 auto'
                justifyContent='center'
                alignItems='center'
            >
                {props.icon}
            </Box>
        </Tooltip>
    );

    const header = props.header && (
        <Tooltip title={props.headerTooltip}>
            <Typography
                variant='h6'
                textAlign='left'
                textOverflow='ellipsis'
                overflow='hidden'
                flexGrow={1}
            >
                {props.header}
            </Typography>
        </Tooltip>
    );

    const subheader = props.subheader && (
        <Tooltip title={props.subheaderTooltip}>
            <Typography
                variant='subtitle1'
                textAlign='left'
                textOverflow='ellipsis'
                overflow='hidden'
                whiteSpace='nowrap'
                fontSize='14px'
                fontWeight={500}
                lineHeight='19px'
            >
                {props.subheader}
            </Typography>
        </Tooltip>
    );

    const body = props.body && (
        <Box flex='1 1 auto'>{props.body}</Box>
    );

    const footer = props.footer && (
        <Box display='flex' alignItems='center' justifyContent='space-between'>
            {props.footer}
        </Box>
    );

    return <CardPaper {...props.paperProps} ref={ref}>
        <Box
            display='flex'
            flexDirection='column'
            justifyContent='space-between'
            height='100%'
            {...props.boxProps}
        >
            <Box flex='0 0 auto' display='flex' gap='15px'>
                {icon}
                <Box flex='1 1 auto'>
                    {header}
                    {subheader}
                </Box>
            </Box>
            {body}
            {footer}
        </Box>
    </CardPaper>
});

CardTemplate.displayName = 'CardTemplate';

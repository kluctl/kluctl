import React from "react";
import { Box, BoxProps, IconButton, Paper, PaperProps, Tooltip } from "@mui/material"
import { CloseLightIcon } from "../../icons/Icons";
import { CardTabs, CardTabsProvider, useCardTabs } from "./CardTabs";
import { ScrollingTextLine } from "../ScrollingTextLine";

export const cardWidth = 247;
export const cardHeight = 126;
export const cardGap = 20;

interface CardPaperProps extends PaperProps {
    glow?: boolean
}

export const CardPaper = React.forwardRef((props: CardPaperProps, ref: React.ForwardedRef<HTMLDivElement>) => {
    const { glow, sx, ...rest } = props;
    let border = `1px solid #59A588`
    let boxShadow = `4px 4px 10px #1E617A`
    if (glow) {
        border = `1px solid lightyellow`
        boxShadow = `0px 0px 40px lightyellow`
    }
    return <Paper
        elevation={5}
        sx={{
            width: '100%',
            height: '100%',
            borderRadius: '12px',
            border: border,
            boxShadow: boxShadow,
            flexShrink: 0,
            position: 'relative',
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
    paperProps?: CardPaperProps,
    boxProps?: BoxProps,
    icon?: React.ReactNode,
    header?: React.ReactNode,
    headerTooltip?: React.ReactNode,
    subheader?: React.ReactNode,
    subheaderTooltip?: React.ReactNode,
    body?: React.ReactNode,
    footer?: React.ReactNode,
    showCloseButton?: boolean,
    onClose?: () => void
}, ref: React.ForwardedRef<HTMLDivElement>) => {
    const icon = props.icon && (
        <Box
            width='45px'
            height='45px'
            flex='0 0 auto'
            justifyContent='center'
            alignItems='center'
        >
            {props.icon}
        </Box>
    );

    const header = props.header && (
        <Tooltip title={props.headerTooltip} placement='bottom-start'>
            <ScrollingTextLine
                variant='h6'
                lineHeight='27px'
                height='27px'
            >
                {props.header}
            </ScrollingTextLine>
        </Tooltip>
    );

    const subheader = props.subheader && (
        <Tooltip title={props.subheaderTooltip} placement='bottom-start'>
            <ScrollingTextLine
                variant='subtitle1'
                fontSize='14px'
                fontWeight={500}
                lineHeight='19px'
                height='19px'
            >
                {props.subheader}
            </ScrollingTextLine>
        </Tooltip>
    );

    const body = props.body && (
        <Box flex='1 1 auto' overflow='hidden' padding='0 16px'>{props.body}</Box>
    );

    const footer = props.footer && (
        <Box display='flex' alignItems='center' justifyContent='space-between'>
            {props.footer}
        </Box>
    );

    return <CardPaper {...props.paperProps} ref={ref}>
        {props.showCloseButton && (
            <Box
                position='absolute'
                right='10px'
                top='10px'
            >
                <IconButton onClick={props.onClose}>
                    <CloseLightIcon />
                </IconButton>
            </Box>
        )}
        <Box
            display='flex'
            flexDirection='column'
            justifyContent='space-between'
            height='100%'
            gap='10px'
            {...props.boxProps}
        >
            <Box flex='0 0 auto' display='flex' gap='15px'>
                {icon}
                <Box
                    flex='1 1 auto'
                    display='flex'
                    flexDirection='column'
                    overflow='hidden'
                    justifyContent='center'
                >
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

export const CardBody = React.memo((props: { provider: CardTabsProvider }) => {
    const { tabs, selectedTab } = useCardTabs(props.provider)

    if (!props.provider
        || !selectedTab
        || !tabs.find(x => x.label === selectedTab)
    ) {
        return null;
    }

    return <CardTabs provider={props.provider}/>
});

CardBody.displayName = 'CardBody';

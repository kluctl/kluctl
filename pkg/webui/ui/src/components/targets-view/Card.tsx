import React from "react";
import { Box, BoxProps, Divider, IconButton, Paper, PaperProps, Tab, Tooltip, Typography } from "@mui/material"
import { CloseLightIcon } from "../../icons/Icons";
import { SidePanelProvider, useSidePanelTabs } from "../result-view/SidePanel";
import { TabContext, TabList, TabPanel } from "@mui/lab";

export const cardWidth = 247;
export const projectCardMinHeight = 80;
export const cardHeight = 126;
export const cardGap = 20;

export const CardPaper = React.forwardRef((props: PaperProps, ref: React.ForwardedRef<HTMLDivElement>) => {
    const { sx, ...rest } = props;
    return <Paper
        elevation={5}
        sx={{
            width: '100%',
            height: '100%',
            borderRadius: '12px',
            border: '1px solid #59A588',
            boxShadow: '4px 4px 10px #1E617A',
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
    paperProps?: PaperProps,
    boxProps?: BoxProps,
    icon?: React.ReactNode,
    iconTooltip?: React.ReactNode,
    header?: string,
    headerTooltip?: React.ReactNode,
    subheader?: string,
    subheaderTooltip?: React.ReactNode,
    body?: React.ReactNode,
    footer?: React.ReactNode,
    showCloseButton?: boolean,
    onClose?: () => void
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
                width='max-content'
            >
                {props.subheader}
            </Typography>
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

export const CardBody = React.memo((props: { provider: SidePanelProvider }) => {
    const { tabs, selectedTab, handleTabChange } = useSidePanelTabs(props.provider)

    if (!props.provider
        || !selectedTab
        || !tabs.find(x => x.label === selectedTab)
    ) {
        return null;
    }

    return <TabContext value={selectedTab}>
        <Box display='flex' flexDirection='column' height='100%' overflow='hidden'>
            <Box height='36px' flex='0 0 auto' p='0'>
                <TabList onChange={handleTabChange}>
                    {tabs.map((tab, i) => {
                        return <Tab label={tab.label} value={tab.label} key={tab.label} />
                    })}
                </TabList>
            </Box>
            <Divider sx={{ margin: 0 }} />
            <Box overflow='auto' p='10px 0'>
                {tabs.map(tab => {
                    return <TabPanel key={tab.label} value={tab.label} sx={{ padding: 0 }}>
                        {tab.content}
                    </TabPanel>
                })}
            </Box>
        </Box>
    </TabContext>
});

CardBody.displayName = 'CardBody';

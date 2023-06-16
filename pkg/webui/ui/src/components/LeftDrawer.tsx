import * as React from 'react';
import { useState } from 'react';
import { CSSObject, styled, Theme, ThemeProvider, useTheme } from '@mui/material/styles';
import Box from '@mui/material/Box';
import MuiDrawer from '@mui/material/Drawer';
import MuiAppBar, { AppBarProps as MuiAppBarProps } from '@mui/material/AppBar';
import List from '@mui/material/List';
import Divider from '@mui/material/Divider';
import IconButton from '@mui/material/IconButton';
import ListItem from '@mui/material/ListItem';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import { Link, useLocation, useNavigate } from "react-router-dom";
import { AppOutletContext } from "./App";
import { ArrowLeftIcon, KluctlLogo, KluctlText, LogoutIcon, SearchIcon, TargetsIcon } from '../icons/Icons';
import { dark } from './theme';
import { Typography } from '@mui/material';

const openedMixin = (theme: Theme): CSSObject => ({
    width: theme.consts.leftDrawerWidthOpen,
    transition: theme.transitions.create('width', {
        easing: theme.transitions.easing.sharp,
        duration: theme.transitions.duration.enteringScreen,
    }),
    overflowX: 'hidden',
});

const closedMixin = (theme: Theme): CSSObject => ({
    transition: theme.transitions.create('width', {
        easing: theme.transitions.easing.sharp,
        duration: theme.transitions.duration.leavingScreen,
    }),
    overflowX: 'hidden',
    width: theme.consts.leftDrawerWidthClosed,
});

const DrawerHeader = styled('div')(({ theme }) => ({
    height: theme.consts.appBarHeight,
    padding: '31px 23px 0 23px',
    // necessary for content to be below app bar
    ...theme.mixins.toolbar,
}));

interface AppBarProps extends MuiAppBarProps {
    open?: boolean;
}

const AppBar = styled(MuiAppBar, {
    shouldForwardProp: (prop) => prop !== 'open',
})<AppBarProps>(({ theme, open }) => ({
    height: theme.consts.appBarHeight,
    border: 'none',
    boxShadow: 'none',
    background: 'transparent',
    padding: '40px 40px 0 40px',
    marginLeft: theme.consts.leftDrawerWidthClosed,
    justifyContent: 'space-between',
    width: `calc(100% - ${theme.consts.leftDrawerWidthClosed}px)`,
    zIndex: theme.zIndex.drawer + 1,
    transition: theme.transitions.create(['width', 'margin'], {
        easing: theme.transitions.easing.sharp,
        duration: theme.transitions.duration.leavingScreen,
    }),
    ...(open && {
        marginLeft: theme.consts.leftDrawerWidthOpen,
        width: `calc(100% - ${theme.consts.leftDrawerWidthOpen}px)`,
        transition: theme.transitions.create(['width', 'margin'], {
            easing: theme.transitions.easing.sharp,
            duration: theme.transitions.duration.enteringScreen,
        }),
    }),
}));

const Drawer = styled(MuiDrawer, { shouldForwardProp: (prop) => prop !== 'open' })(
    ({ theme, open }) => ({
        width: theme.consts.leftDrawerWidthOpen,
        flexShrink: 0,
        whiteSpace: 'nowrap',
        boxSizing: 'border-box',
        borderRadius: '0px 20px 20px 0px',
        ...(open && {
            ...openedMixin(theme),
            '& .MuiDrawer-paper': {
                ...openedMixin(theme),
                borderRadius: '0px 20px 20px 0px',
            }
        }),
        ...(!open && {
            ...closedMixin(theme),
            '& .MuiDrawer-paper': {
                ...closedMixin(theme),
                borderRadius: '0px 20px 20px 0px',
            }
        }),
    }),
);

function Item(props: { text: string, open: boolean, icon: React.ReactNode, to?: string, selected?: boolean, onClick?: () => void }) {
    const linkProps = props.to ? { component: Link, to: props.to } : undefined;
    return <ListItem disablePadding sx={{ display: 'block', margin: '14px 0' }} onClick={props.onClick} {...linkProps}>
        <ListItemButton
            sx={{
                height: '60px',
                justifyContent: 'start',
                alignItems: 'center',
                gap: '15px',
                padding: '0 24px'
            }}
        >
            <ListItemIcon
                sx={{
                    minWidth: 0,
                    width: '48px',
                    height: '48px',
                    flex: '0 0 auto',
                    justifyContent: 'center',
                    alignItems: 'center',
                }}
            >
                {props.icon}
            </ListItemIcon>
            <ListItemText
                primary={props.text}
                sx={{ display: props.open ? 'block' : 'none', margin: 0 }}
                primaryTypographyProps={{
                    fontWeight: 500,
                    fontSize: '24px',
                    lineHeight: '33px',
                    letterSpacing: '1px',
                    color: props.selected ? undefined : '#8A8E91'
                }}
            />
        </ListItemButton>
    </ListItem>
}

export default function LeftDrawer(props: {
    content: React.ReactNode,
    context: AppOutletContext,
    logout: () => void
}) {
    const [open, setOpen] = useState(true);
    const location = useLocation()
    const navigate = useNavigate();
    const theme = useTheme();

    const toggleDrawer = () => {
        setOpen(o => !o);
    };

    const path = location.pathname.split('/')[1];
    let header: JSX.Element | null = null;
    switch (path) {
        case 'targets':
            header = <>
                <Typography variant='h1'>
                    Dashboard
                </Typography>
                <Box
                    height='40px'
                    maxWidth='314px'
                    flexGrow={1}
                    borderRadius='10px'
                    display='flex'
                    justifyContent='space-between'
                    alignItems='center'
                    padding='0 9px 0 15px'
                    sx={{ background: theme.palette.background.default }}
                >
                    <input
                        type='text'
                        style={{
                            background: 'none',
                            border: 'none',
                            outline: 'none',
                            height: '20px',
                            lineHeight: '20px',
                            fontSize: '18px'
                        }}
                        placeholder='Search (not impl. yet)'
                    />
                    <IconButton sx={{ padding: 0, height: 40, width: 40 }}>
                        <SearchIcon />
                    </IconButton>
                </Box>
            </>
            break;
        case 'results':
            header = <Box
                display='flex'
                alignItems='center'
                justifyContent='start'
                gap='12px'
            >
                <IconButton sx={{ padding: 0 }} onClick={() => navigate('/targets')}>
                    <ArrowLeftIcon />
                </IconButton>
                <Typography variant='h1'>
                    Result Tree
                </Typography>
            </Box>
            break;
    }

    return (
        <Box display='flex' height='100%'>
            <AppBar position='fixed' open={open}>
                <Box display='flex' justifyContent='space-between'>
                    {header}
                </Box>
            </AppBar>
            <ThemeProvider theme={dark}>
                <Drawer variant="permanent" open={open}>
                    <DrawerHeader>
                        <IconButton
                            onClick={toggleDrawer}
                            sx={{ gap: '13px', padding: 0 }}>
                            <KluctlLogo />
                            {open && <KluctlText />}
                        </IconButton>
                    </DrawerHeader>
                    <Divider />
                    <Box display='flex' flexDirection='column' flex='1 1 auto' pt='10px'>
                        <Box flex='1 1 auto'>
                            <List sx={{ padding: 0 }}>
                                <Item text={"Targets"} open={open} icon={<TargetsIcon />} to={"targets"} selected />
                            </List>
                        </Box>
                        <Box flex='0 0 auto'>
                            <List sx={{ padding: 0 }}>
                                <Item text={"Log Out"} open={open} icon={<LogoutIcon />} onClick={props.logout} />
                            </List>
                        </Box>
                    </Box>
                </Drawer>
            </ThemeProvider>
            <Box component="main" sx={{ flexGrow: 1, height: '100%', overflow: 'hidden' }} minWidth={0}>
                <DrawerHeader />
                <Divider sx={{ margin: '0 40px' }} />
                <Box width='100%' height={`calc(100% - ${theme.consts.appBarHeight}px)`} overflow='auto'>
                    {props.content}
                </Box>
            </Box>
        </Box>
    );
}

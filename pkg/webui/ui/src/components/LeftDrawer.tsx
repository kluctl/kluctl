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
import { Link } from "react-router-dom";
import { AppOutletContext } from "./App";
import { KluctlLogo, TargetsIcon, KluctlText, SearchIcon } from '../icons/Icons';
import { dark } from './theme';
import { Typography } from '@mui/material';

const drawerWidthOpen = 224;
const drawerWidthClosed = 96;

const openedMixin = (theme: Theme): CSSObject => ({
    width: drawerWidthOpen,
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
    width: drawerWidthClosed,
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
    marginLeft: drawerWidthClosed,
    justifyContent: 'space-between',
    width: `calc(100% - ${drawerWidthClosed}px)`,
    zIndex: theme.zIndex.drawer + 1,
    transition: theme.transitions.create(['width', 'margin'], {
        easing: theme.transitions.easing.sharp,
        duration: theme.transitions.duration.leavingScreen,
    }),
    ...(open && {
        marginLeft: drawerWidthOpen,
        width: `calc(100% - ${drawerWidthOpen}px)`,
        transition: theme.transitions.create(['width', 'margin'], {
            easing: theme.transitions.easing.sharp,
            duration: theme.transitions.duration.enteringScreen,
        }),
    }),
}));

const Drawer = styled(MuiDrawer, { shouldForwardProp: (prop) => prop !== 'open' })(
    ({ theme, open }) => ({
        width: drawerWidthOpen,
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

function Item(props: { text: string, open: boolean, icon: React.ReactNode, to: string }) {
    return <ListItem component={Link} to={props.to} key={props.text} disablePadding sx={{ display: 'block' }}>
        <ListItemButton
            sx={{
                minHeight: 48,
                justifyContent: props.open ? 'initial' : 'center',
                px: '24px',
            }}
        >
            <ListItemIcon
                sx={{
                    minWidth: 0,
                    mr: props.open ? 3 : 'auto',
                    justifyContent: 'center',
                }}
            >
                {props.icon}
            </ListItemIcon>
            <ListItemText primary={props.text} sx={{ opacity: props.open ? 1 : 0 }} />
        </ListItemButton>
    </ListItem>
}

export default function LeftDrawer(props: { content: React.ReactNode, context: AppOutletContext }) {
    const context = props.context

    const [open, setOpen] = useState(true);

    const theme = useTheme();

    const toggleDrawer = () => {
        setOpen(o => !o);
    };

    return (
        <Box display='flex' height='100%'>
            <AppBar position='fixed' open={open}>
                <Box display='flex' justifyContent='space-between'>
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
                            placeholder='Search'
                        />
                        <IconButton sx={{ padding: 0, height: 40, width: 40 }}>
                            <SearchIcon />
                        </IconButton>
                    </Box>
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
                    <List sx={{ padding: 0 }}>
                        <Item text={"Targets"} open={open} icon={<TargetsIcon />} to={"targets"} />
                        <Divider />
                        {context.filters}
                    </List>
                    {context.filters && <Divider />}
                </Drawer>
            </ThemeProvider>
            <Box component="main" sx={{ flexGrow: 1, height: '100%', overflow: 'hidden' }} minWidth={0}>
                <DrawerHeader />
                <Divider sx={{ margin: '0 40px' }}/>
                <Box width='100%' height={`calc(100% - ${theme.consts.appBarHeight}px)`} overflow='auto'>
                    {props.content}
                </Box>
            </Box>
        </Box>
    );
}

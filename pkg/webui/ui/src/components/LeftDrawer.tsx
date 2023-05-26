import * as React from 'react';
import { useState } from 'react';
import { CSSObject, styled, Theme, ThemeProvider, useTheme } from '@mui/material/styles';
import Box from '@mui/material/Box';
import MuiDrawer from '@mui/material/Drawer';
import MuiAppBar, { AppBarProps as MuiAppBarProps } from '@mui/material/AppBar';
import Toolbar from '@mui/material/Toolbar';
import List from '@mui/material/List';
import Divider from '@mui/material/Divider';
import IconButton from '@mui/material/IconButton';
import MenuIcon from '@mui/icons-material/Menu';
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import ListItem from '@mui/material/ListItem';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemIcon from '@mui/material/ListItemIcon';
import ListItemText from '@mui/material/ListItemText';
import { Link } from "react-router-dom";
import { AppOutletContext } from "./App";
import { KluctlLogo } from "../icons/KluctlLogo";
import { Targets } from '../icons/Targets';
import { dark } from './theme';
import { KluctlText } from '../icons/KluctlText';
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
    height: '106px',
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
    height: 106,
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

    const toggleDrawer = () => {
        setOpen(o => !o);
    };

    return (
        <Box sx={{ display: 'flex' }} height={"100%"}>
            <AppBar position="fixed" open={open}>
                <Typography variant='h1' fontSize='32px' fontWeight='bold'>
                    Dashboard
                </Typography>
                <Divider />
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
                    <List>
                        <Item text={"Targets"} open={open} icon={<Targets />} to={"targets"} />
                        <Divider />
                        {context.filters}
                    </List>
                    {context.filters && <Divider />}
                </Drawer>
            </ThemeProvider>
            <Box component="main" sx={{ flexGrow: 1 }} minWidth={0}>
                <DrawerHeader />
                {props.content}
            </Box>
        </Box>
    );
}

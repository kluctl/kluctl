import React from "react";
import { MoreVert } from "@mui/icons-material";
import { IconButton, Menu, MenuItem } from "@mui/material";
import { Link } from "react-router-dom";

export interface ActionMenuItem {
    icon: React.ReactNode
    text: string
    handler?: () => void
    to?: string
}

export const ActionsMenu = (props: { icon?: React.ReactNode, menuItems: ActionMenuItem[] }) => {
    const [anchorEl, setAnchorEl] = React.useState<null | HTMLElement>(null);
    const menuOpen = Boolean(anchorEl);

    const handleMenuClick = (e: React.MouseEvent<HTMLElement>) => {
        e.stopPropagation()
        setAnchorEl(e.currentTarget);
    };
    const handleMenuClose = () => {
        setAnchorEl(null);
    };

    const icon = props.icon || <MoreVert/>

    return <React.Fragment>
        <IconButton
            onClick={handleMenuClick}
            size="small"
            aria-controls={menuOpen ? 'account-menu' : undefined}
            aria-haspopup="true"
            aria-expanded={menuOpen ? 'true' : undefined}
        >
            {icon}
        </IconButton>
        <Menu
            anchorEl={anchorEl}
            id="account-menu"
            open={menuOpen}
            onClose={handleMenuClose}
            onClick={ e => {e.stopPropagation(); handleMenuClose()}}
            PaperProps={{
                elevation: 0,
                sx: {
                    overflow: 'visible',
                    filter: 'drop-shadow(0px 2px 8px rgba(0,0,0,0.32))',
                    mt: 1.5,
                    '& .MuiAvatar-root': {
                        width: 32,
                        height: 32,
                        ml: -0.5,
                        mr: 1,
                    },
                    '&:before': {
                        content: '""',
                        display: 'block',
                        position: 'absolute',
                        top: 0,
                        right: 14,
                        width: 10,
                        height: 10,
                        bgcolor: 'background.paper',
                        transform: 'translateY(-50%) rotate(45deg)',
                        zIndex: 0,
                    },
                },
            }}
            transformOrigin={{ horizontal: 'right', vertical: 'top' }}
            anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
        >
            {props.menuItems.map((item, i) => {
                const handler = (e: React.SyntheticEvent) => {
                    e.stopPropagation()
                    if (item.handler) {
                        item.handler()
                    }
                    handleMenuClose()
                }
                if (item.to) {
                    return <MenuItem key={i} component={Link} to={item.to} onClick={handler}>
                        {item.icon} {item.text}
                    </MenuItem>
                } else {
                    return <MenuItem key={i} onClick={handler}>
                        {item.icon} {item.text}
                    </MenuItem>
                }
            })}

        </Menu>
    </React.Fragment>
}

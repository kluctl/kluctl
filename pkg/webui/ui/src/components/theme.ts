import { PaletteOptions, createTheme } from '@mui/material/styles';

const paletteDark = {
    primary: { main: '#DFEBE9' },
    background: { default: '#222222', paper: '#222222' }
} satisfies PaletteOptions;

const paletteLight = {
    primary: { main: '#222222' },
    secondary: { main: '#39403E' },
    background: { default: '#DFEBE9', paper: '#DFEBE9' },
    text: { primary: '#222222' }
} satisfies PaletteOptions;

export const light = createTheme({
    palette: paletteLight,
    typography: {
        fontFamily: 'Nunito Variable',
        h1: { color: '#222222' },
        h6: { color: '#39403E' },
        subtitle1: { color: '#39403E' },
        subtitle2: { fontSize: '14px', lineHeight: 1.2 }
    },
    components: {
        MuiDivider: {
            styleOverrides: {
                root: {
                    borderColor: 'rgba(0, 0, 0, 0.5)'
                }
            }
        },
        MuiAppBar: {
            styleOverrides: {
                root: {
                    color: paletteLight.primary.main
                }
            }
        }
    }
});

export const dark = createTheme({
    palette: paletteDark,
    typography: {
        fontFamily: 'Nunito Variable'
    },
    components: {
        MuiListItem: {
            styleOverrides: {
                root: {
                    color: paletteDark.primary.main,
                    background: paletteDark.background.default
                }
            }
        },
        MuiButtonBase: {
            styleOverrides: {
                root: {
                    color: paletteDark.primary.main,
                    background: paletteDark.background.default
                }
            }
        },
        MuiDrawer: {
            styleOverrides: {
                root: {
                    border: 'none'
                },
                paper: {
                    border: 'none'
                },
            }
        },
        MuiDivider: {
            styleOverrides: {
                root: {
                    background: paletteDark.primary.main,
                    opacity: 0.2,
                    margin: '0 13px'
                }
            }
        }
    }
})
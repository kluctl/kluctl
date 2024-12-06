import { createTheme, PaletteOptions, ThemeOptions } from '@mui/material/styles';

const paletteDark = {
    primary: { main: '#DFEBE9' },
    background: { default: '#222222', paper: '#222222' },
    text: { primary: '#DFEBE9' }
} satisfies PaletteOptions;

const paletteLight = {
    primary: { main: '#222222' },
    secondary: { main: '#39403E' },
    background: { default: '#DFEBE9', paper: '#DFEBE9' },
    text: { primary: '#222222' }
} satisfies PaletteOptions;

declare module '@mui/material/styles' {
    interface Theme {
        consts: {
            appBarHeight: number;
            leftDrawerWidthOpen: number;
            leftDrawerWidthClosed: number;
        };
    }
    // allow configuration using `createTheme`
    interface ThemeOptions {
        consts?: {
            appBarHeight?: number;
            leftDrawerWidthOpen?: number;
            leftDrawerWidthClosed?: number;
        };
    }
}

export const common = createTheme({
    consts: {
        appBarHeight: 106,
        leftDrawerWidthOpen: 224,
        leftDrawerWidthClosed: 96
    },
    typography: {
        fontFamily: 'Nunito Variable',
    },
    components: {
        MuiBackdrop: {
            styleOverrides: {
                root: {
                    backgroundColor: 'rgba(0, 0, 0, 0.65)'
                },
                invisible: {
                    backgroundColor: 'transparent'
                }
            }
        }
    }
});

export const light = createTheme(common, {
    palette: paletteLight,
    typography: {
        h1: {
            color: paletteLight.text.primary,
            fontWeight: 700,
            fontSize: '32px',
            lineHeight: '44px',
            letterSpacing: '1px',
        },
        h2: {
            color: paletteLight.text.primary,
            fontWeight: 700,
            fontSize: '20px',
            lineHeight: '27px',
            letterSpacing: '1px',
        },
        h5: {
            color: paletteLight.secondary.main,
            fontWeight: 700,
            fontSize: '22px',
            lineHeight: '30px',
            letterSpacing: '1px',
        },
        h6: {
            color: paletteLight.secondary.main,
            fontWeight: 800,
            fontSize: '20px',
            lineHeight: '27px'
        },
        subtitle1: { color: paletteLight.secondary.main },
        subtitle2: { fontSize: '14px', lineHeight: 1.2 }
    },
    components: {
        MuiDivider: {
            styleOverrides: {
                root: {
                    borderColor: paletteLight.secondary.main,
                }
            }
        },
        MuiTabs: {
            styleOverrides: {
                root: {
                    height: '36px',
                    minHeight: 0,
                    textTransform: 'none'
                },
                indicator: {
                    backgroundColor: '#59A588'
                }
            }
        },
        MuiTab: {
            styleOverrides: {
                root: {
                    height: '36px',
                    minHeight: 0,
                    fontWeight: 400,
                    fontSize: '16px',
                    lineHeight: '22px',
                    letterSpacing: '1px',
                    textTransform: 'none',
                    padding: '7px 5px',
                    color: '#8A8E91',
                    '&.Mui-selected': {
                        color: paletteLight.text.primary
                    }
                }
            }
        },
        MuiAppBar: {
            styleOverrides: {
                root: {
                    color: paletteLight.primary.main
                }
            }
        },
        MuiTableCell: {
            styleOverrides: {
                root: {
                    border: 'none',
                    borderTop: `0.5px solid ${paletteLight.secondary.main}`,
                    fontWeight: 400,
                    fontSize: '16px',
                    lineHeight: '22px',
                    letterSpacing: '1px',
                    position: 'relative',
                    ':after': {
                        content: '""',
                        position: 'absolute',
                        top: '10px',
                        right: 0,
                        bottom: '10px',
                        display: 'block',
                        borderRight: `0.5px solid ${paletteLight.secondary.main}`,
                    },
                    ':last-of-type:after': {
                        content: 'none'
                    },
                    ':last-of-type': {
                        overflowWrap: 'anywhere'
                    }
                },
                head: {
                    border: 'none',
                }
            }
        }
    }
} satisfies ThemeOptions);

export const dark = createTheme(common, {
    palette: paletteDark,
    typography: {
        allVariants: {
            color: paletteDark.text.primary
        },
        h4: {
            color: paletteDark.text.primary,
            fontWeight: 700,
            fontSize: '24px',
            lineHeight: '33px',
            letterSpacing: '1px',
        }
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
        },
        MuiTabs: {
            styleOverrides: {
                root: {
                    height: '36px',
                    minHeight: 0,
                    textTransform: 'none'
                },
                indicator: {
                    backgroundColor: '#59A588'
                }
            }
        },
        MuiTab: {
            styleOverrides: {
                root: {
                    height: '36px',
                    minHeight: 0,
                    fontWeight: 400,
                    fontSize: '16px',
                    lineHeight: '22px',
                    letterSpacing: '1px',
                    textTransform: 'none',
                    padding: '7px 5px',
                    color: '#8A8E91',
                    '&.Mui-selected': {
                        color: paletteDark.text.primary
                    }
                }
            }
        }
    }
} satisfies ThemeOptions)
import { Box, Button, FormControl, Paper, TextField, Typography, useTheme } from '@mui/material';
import React, { SyntheticEvent, useState } from 'react';
import { rootPath } from "../api";
import { AuthInfo } from "../models";

interface loginCredentials {
    username: string
    password: string
}

type LoginResult = {
    type: 'success',
} | {
    type: 'error',
    statusText: string
}

async function loginStaticUser(creds: loginCredentials): Promise<LoginResult> {
    const url = rootPath + `/auth/staticLogin`
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(creds)
        })
        if (response.status === 200) {
            return { type: "success" }
        } else if (response.status === 401) {
            let statusText = await response.text()
            if (!statusText) {
                statusText = "Invalid username/password"
            }
            return { type: 'error', statusText: statusText }
        } else {
            return {
                type: 'error',
                statusText: response.statusText
            }
        }
    } catch (e) {
        return {
            type: 'error',
            statusText: "unknown error",
        }
    }
}

export default function Login(props: { authInfo: AuthInfo }) {
    const theme = useTheme();
    const [username, setUserName] = useState<string>();
    const [password, setPassword] = useState<string>();

    const searchParams = new URLSearchParams(window.location.search)
    const loginErrorFromSearchParams = searchParams.get("login_error")
    let initialError: LoginResult | undefined
    if (loginErrorFromSearchParams) {
        initialError = {
            type: "error",
            statusText: loginErrorFromSearchParams,
        }
    }

    const [error, setError] = useState<LoginResult | undefined>(initialError);

    const handleSubmit = async (e: SyntheticEvent) => {
        e.preventDefault();
        if (!username || !password) {
            return
        }

        const result = await loginStaticUser({
            username: username,
            password: password,
        });

        switch (result.type) {
            case 'success':
                window.location.href='/'
                setError(undefined)
                break;
            case 'error':
                setError(result)
                break;
        }
    }

    const staticLogin = <form onSubmit={handleSubmit}>
        <Box
            display='flex'
            flexDirection='column'
            justifyContent='center'
            alignItems='center'
            gap='30px'
        >
            <FormControl fullWidth >
                <TextField variant='standard' label='Username' onChange={e => setUserName(e.target.value)} />
            </FormControl>
            <FormControl fullWidth >
                <TextField variant='standard' label='Password' type="password" onChange={e => setPassword(e.target.value)} />
            </FormControl>
            {error?.type !== 'success' && (
                <Box color={theme.palette.error.main}>
                    {error?.statusText}
                </Box>
            )}
            <FormControl >
                <Button
                    variant='contained'
                    type='submit'
                    sx={{
                        padding: '6px 20px',
                        backgroundColor: '#59A588',
                        '&:hover': {
                            backgroundColor: '#59A588'
                        }
                    }}
                >
                    Login
                </Button>
            </FormControl>
        </Box>
    </form>

    const oidcLogin = <Box
        display='flex'
        flexDirection='column'
        justifyContent='center'
        alignItems='center'
        gap='30px'
    >
        <Button
            variant='contained'
            type='submit'
            sx={{
                padding: '6px 20px',
                backgroundColor: '#59A588',
                '&:hover': {
                    backgroundColor: '#59A588'
                }
            }}
            onClick={() => {
                window.location.href='/auth/login'
            }}
        >
            Login with {props.authInfo.oidcName}
        </Button>
    </Box>

    return (
        <Box
            width='100%'
            height='100%'
            display='flex'
            flexDirection='column'
            alignItems='center'
            gap='30px'
            pt='90px'
        >
            <Typography variant='h1'>Please Log In</Typography>
            <Paper elevation={5} sx={{ width: '400px', borderRadius: '12px', padding: '30px' }}>
                {props.authInfo.staticLoginEnabled && staticLogin}
                {props.authInfo.oidcEnabled && oidcLogin}
            </Paper>
        </Box>
    )
}

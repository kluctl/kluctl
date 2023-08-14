import { Box, Button, FormControl, Paper, TextField, Typography, useTheme } from '@mui/material';
import React, { SyntheticEvent, useState } from 'react';
import { rootPath } from "../api";
import { ErrorMessageCard } from './ErrorMessage';
import { AuthInfo } from "../models";

interface loginCredentials {
    username: string
    password: string
}

type LoginResult = {
    type: 'success',
} | {
    type: 'unauthorized'
} | {
    type: 'other-error',
    statusText: string
}

async function loginStaticUser(creds: loginCredentials): Promise<LoginResult> {
    const url = rootPath + `/auth/staticLogin`
    return fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(creds)
    }).then(response => {
        if (response.status === 200) {
            return { type: "success" }
        } else if (response.status === 401) {
            return { type: 'unauthorized' }
        } else {
            return {
                type: 'other-error',
                statusText: response.statusText
            }
        }
    });
}

export default function Login(props: { authInfo: AuthInfo }) {
    const theme = useTheme();
    const [username, setUserName] = useState<string>();
    const [password, setPassword] = useState<string>();
    const [error, setError] = useState<Exclude<LoginResult, { type: 'success' }>>();

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
                break;
            case 'unauthorized':
                setError(result);
                break;
            case 'other-error':
                setError(result)
                break;
        }
    }

    if (error?.type === 'other-error') {
        return <ErrorMessageCard>
            {error.statusText}
        </ErrorMessageCard>
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
            {error?.type === 'unauthorized' && (
                <Box color={theme.palette.error.main}>
                    Invalid username or password
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

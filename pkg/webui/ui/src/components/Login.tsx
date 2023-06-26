import { Box, Button, FormControl, Paper, TextField, Typography, useTheme } from '@mui/material';
import React, { SyntheticEvent, useState } from 'react';
import { rootPath } from "../api";
import { ErrorMessageCard } from './ErrorMessage';

//import './Login.css';

interface loginCredentials {
    username: string
    password: string
}

type LoginResult = {
    type: 'success',
    token: string
} | {
    type: 'unauthorized'
} | {
    type: 'other-error',
    statusText: string
}

async function loginUser(creds: loginCredentials): Promise<LoginResult> {
    const url = rootPath + `/auth/login`
    return fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(creds)
    }).then(response => {
        if (response.status === 200) {
            return response.json().then(json => ({
                type: 'success',
                token: json.token
            }));
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

export default function Login(props: { setToken: (token: string) => void }) {
    const theme = useTheme();
    const [username, setUserName] = useState<string>();
    const [password, setPassword] = useState<string>();
    const [error, setError] = useState<Exclude<LoginResult, { type: 'success' }>>();

    const handleSubmit = async (e: SyntheticEvent) => {
        e.preventDefault();
        if (!username || !password) {
            return
        }

        const result = await loginUser({
            username: username,
            password: password,
        });

        switch (result.type) {
            case 'success':
                props.setToken(result.token);
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
                <form onSubmit={handleSubmit}>
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
            </Paper>
        </Box>
    )
}

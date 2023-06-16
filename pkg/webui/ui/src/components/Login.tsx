import { Box, Button, FormControl, Paper, TextField, Typography } from '@mui/material';
import React, { SyntheticEvent, useState } from 'react';

//import './Login.css';

interface loginCredentials {
    username: string
    password: string
}

async function loginUser(creds: loginCredentials) {
    const url = `/auth/login`
    return fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(creds)
    }).then(data => data.json())
        .then(token => token.token)
}

export default function Login(props: { setToken: (token: string) => void }) {
    const [username, setUserName] = useState<string>();
    const [password, setPassword] = useState<string>();

    const handleSubmit = async (e: SyntheticEvent) => {
        e.preventDefault();
        if (!username || !password) {
            return
        }
        const token = await loginUser({
            username: username,
            password: password,
        });
        props.setToken(token);
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

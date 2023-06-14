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

export default function Login(props: { setToken: (token: string) => void}) {
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

    return(
        <div className="login-wrapper">
            <h1>Please Log In</h1>
            <form onSubmit={handleSubmit}>
                <label>
                    <p>Username</p>
                    <input type="text" onChange={e => setUserName(e.target.value)} />
                </label>
                <label>
                    <p>Password</p>
                    <input type="password" onChange={e => setPassword(e.target.value)} />
                </label>
                <div>
                    <button type="submit">Submit</button>
                </div>
            </form>
        </div>
    )
}

import React from 'react';
import ReactDOM from 'react-dom/client';
import { RouterProvider } from "react-router-dom";

import './index.css';
import reportWebVitals from './reportWebVitals';

import '@fontsource-variable/nunito';

import { Router } from "./components/Router";
import { ThemeProvider } from '@mui/material';
import { light } from './components/theme';

const root = ReactDOM.createRoot(
    document.getElementById('root') as HTMLElement
);
root.render(
    <React.StrictMode>
        <ThemeProvider theme={light}>
            <RouterProvider router={Router} />
        </ThemeProvider>
    </React.StrictMode>
);

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals();

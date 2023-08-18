import React from 'react';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';

export interface PropertiesEntry {
    name: string
    value: React.ReactNode
}

export const pushProp = (props: PropertiesEntry[], name: string, v?: any, render?: () => React.ReactElement ) => {
    if (!v) {
        return
    }
    if (render) {
        props.push({ name: name, value: render() })
    } else {
        props.push({ name: name, value: v + "" })
    }
}

export function PropertiesTable(props: {properties: PropertiesEntry[]}) {
    return (
        <TableContainer>
            <Table>
                <TableHead>
                    <TableRow>
                        <TableCell>Name</TableCell>
                        <TableCell>Value</TableCell>
                    </TableRow>
                </TableHead>
                <TableBody>
                    {props.properties.map((row) => (
                        <TableRow key={row.name}>
                            <TableCell>{row.name}</TableCell>
                            <TableCell>{row.value}</TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        </TableContainer>
    );
}
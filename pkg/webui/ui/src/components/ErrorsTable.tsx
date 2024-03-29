import React from 'react';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import { DeploymentError } from "../models";
import { Box, Divider, List, ListItem, ListItemText } from "@mui/material";
import { buildRefKindElement } from "../api";
import { buildListKey } from "../utils/listKey";

export function ErrorsTable(props: { errors: DeploymentError[] }) {
    return <>
        <Box height={"100%"}>
            <TableContainer>
                <Table>
                    <TableHead>
                        <TableRow>
                            <TableCell>Ref</TableCell>
                            <TableCell>Message</TableCell>
                        </TableRow>
                    </TableHead>
                    <TableBody>
                        {props.errors?.map(e=> (
                            <TableRow key={buildListKey(e)}>
                                <TableCell sx={{ minWidth: "150px" }}>
                                    <List disablePadding>
                                        {buildRefKindElement(e.ref, <>
                                            <ListItem>
                                                <ListItemText primary={e.ref.kind}/>
                                            </ListItem>
                                        </>)}
                                        <Divider/>
                                        <ListItem>
                                            <ListItemText primary={e.ref.name}/>
                                        </ListItem>
                                        {e.ref.namespace && <>
                                            <Divider/>
                                            <ListItem>
                                                <ListItemText primary={e.ref.namespace}/>
                                            </ListItem>
                                        </>}
                                    </List>
                                </TableCell>
                                <TableCell>
                                    {e.message}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </TableContainer>
        </Box>
    </>
}
import React from 'react';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import { ValidateResultEntry } from "../models";
import { Box, Divider, List, ListItem, ListItemText, Tooltip, Typography } from "@mui/material";
import { buildRefKindElement } from "../api";
import { buildListKey } from "../utils/listKey";
import ReactMarkdown from "react-markdown";
import remarkGfm from 'remark-gfm';

export function ValidateResultsTable(props: { results: ValidateResultEntry[] }) {
    return <>
        <Box height={"100%"}>
            <TableContainer>
                <Table>
                    <TableHead>
                        <TableRow>
                            <TableCell>Ref</TableCell>
                            <TableCell>Annotation</TableCell>
                            <TableCell>Message</TableCell>
                        </TableRow>
                    </TableHead>
                    <TableBody>
                        {props.results?.map(e=> (
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
                                    <Tooltip title={e.annotation}>
                                        <Typography>{e.annotation.replace("validate-result.kluctl.io/", "")}</Typography>
                                    </Tooltip>
                                </TableCell>
                                <TableCell>
                                    <ReactMarkdown remarkPlugins={[remarkGfm]} components={{
                                        // make sure links are opened in a new tab
                                        a({ node, children, ...props }) {
                                            let url = new URL(props.href ?? "", window.location.href);
                                            if (url.origin !== window.location.origin) {
                                                props.target = "_blank";
                                                props.rel = "noopener noreferrer";
                                            }

                                            return <a {...props}>{children}</a>;
                                        },
                                    }}>
                                        {e.message}
                                    </ReactMarkdown>
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </TableContainer>
        </Box>
    </>
}
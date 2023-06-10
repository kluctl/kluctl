import React from 'react';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import { buildRefKindElement, buildRefString } from "../../api";
import { Box, Typography } from "@mui/material";
import { CodeViewer } from "../CodeViewer";
import { DiffStatus } from "./nodes/NodeData";
import { ObjectRef } from "../../models";
import { buildListKey } from "../../utils/listKey";

const RefList = (props: { title: string, refs: ObjectRef[] }) => {
    return <Box py='10px'>
        <Typography align={"center"} variant={"h5"}>{props.title}</Typography>
        <TableContainer>
            <Table>
                <TableHead>
                    <TableRow>
                        <TableCell align="left">
                            <Typography>Kind</Typography>
                        </TableCell>
                        <TableCell align="left">
                            <Typography>Namespace</Typography>
                        </TableCell>
                        <TableCell align="left">
                            <Typography>Name</Typography>
                        </TableCell>
                    </TableRow>
                </TableHead>
                <TableBody>
                    {props.refs.map(ref => {
                        return <TableRow key={buildRefString(ref)}>
                            <TableCell>
                                {buildRefKindElement(ref)}
                            </TableCell>
                            <TableCell>
                                <Typography>{ref.namespace}</Typography>
                            </TableCell>
                            <TableCell>
                                <Typography>{ref.name}</Typography>
                            </TableCell>
                        </TableRow>
                    })}
                </TableBody>
            </Table>
        </TableContainer>
    </Box>
}

export function ChangesTable(props: { diffStatus: DiffStatus }) {
    let changedObjects: React.ReactElement | undefined

    if (props.diffStatus.changedObjects.length) {
        changedObjects = <Box py='10px'>
            <Typography align={"center"} variant={"h5"}>Changed Objects</Typography>
            {props.diffStatus.changedObjects.map(co => (
                <TableContainer key={buildRefString(co.ref)}>
                    <Table>
                        <TableHead>
                            <TableRow>
                                <TableCell align="left" colSpan={2} sx={{ padding: '24px 16px 5px 16px' }}>
                                    <Typography
                                        sx={{
                                            fontWeight: 500,
                                            fontSize: '20px',
                                            lineHeight: '27px',
                                            letterSpacing: '1px',
                                        }}
                                    >{buildRefString(co.ref)}</Typography>
                                </TableCell>
                            </TableRow>
                            <TableRow>
                                <TableCell>Path</TableCell>
                                <TableCell>Changes</TableCell>
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            {co.changes?.map(c => (
                                <TableRow key={buildListKey(c)}>
                                    <TableCell>
                                        <Box minWidth={"100px"} sx={{ overflowWrap: "anywhere" }}>
                                            <Typography>{c.jsonPath}</Typography>
                                        </Box>
                                    </TableCell>
                                    <TableCell>
                                        <CodeViewer code={c.unifiedDiff || ""} language={"diff"} />
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </TableContainer>
            ))
            }
        </Box >
    }

    return <Box width={"100%"} display={"flex"} flexDirection={"column"}>
        {props.diffStatus.newObjects.length ?
            <RefList title={"New Objects"} refs={props.diffStatus.newObjects} /> : <></>}
        {props.diffStatus.deletedObjects.length ?
            <RefList title={"Deleted Objects"} refs={props.diffStatus.deletedObjects} /> : <></>}
        {props.diffStatus.orphanObjects.length ?
            <RefList title={"Orphan Objects"} refs={props.diffStatus.orphanObjects} /> : <></>}
        {changedObjects}
    </Box>
}
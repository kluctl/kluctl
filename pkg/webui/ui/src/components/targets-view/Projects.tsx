import { ProjectSummary } from "../../models";
import { getLastPathElement } from "../../utils/misc";
import Paper from "@mui/material/Paper";
import { Box, Typography } from "@mui/material";
import React from "react";
import Tooltip from "@mui/material/Tooltip";
import { ProjectIcon } from "../../icons/Icons";

export const ProjectItem = (props: { ps: ProjectSummary }) => {
    const name = getLastPathElement(props.ps.project.gitRepoKey)
    const subDir = props.ps.project.subDir

    const projectInfo = <Box>
        {props.ps.project.gitRepoKey}<br />
        {props.ps.project.subDir ? <>
            SubDir: {props.ps.project.subDir}<br />
        </> : <></>}
    </Box>

    return <Paper
        elevation={5}
        sx={{
            width: "100%",
            height: "100%",
            borderRadius: '12px',
            border: '1px solid #59A588',
            boxShadow: '4px 4px 10px #1E617A',
            padding: '5px 16px'
        }}>
        <Box display="flex" flexDirection={"column"} justifyContent='center' height={"100%"}>
            <Box display='flex' alignItems='center' gap='15px'>
                {name && (
                    <>
                        <Box flexShrink={0}><ProjectIcon /></Box>
                        <Tooltip title={projectInfo}>
                            <Typography
                                variant='h6'
                                textAlign='left'
                                textOverflow='ellipsis'
                                overflow='hidden'
                                lineHeight={1.2}
                                flexGrow={1}
                                fontSize='20px'
                                fontWeight={800}
                            >
                                {name}
                            </Typography>
                        </Tooltip>
                    </>
                )}
            </Box>
            <Box>
                {subDir ?
                    <Typography textAlign={"center"}>{subDir}</Typography>
                    : <></>}
            </Box>
            <Box>
                <Box display={"flex"}>
                    <Box marginLeft={"auto"}>
                        {/*<ActionsMenu menuItems={actionMenuItems}/>*/}
                    </Box>
                </Box>
            </Box>
        </Box>
    </Paper>
}

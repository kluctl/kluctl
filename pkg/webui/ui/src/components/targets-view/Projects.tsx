import { ProjectSummary } from "../../models";
import { getLastPathElement } from "../../utils/misc";
import Paper from "@mui/material/Paper";
import { Box, Typography } from "@mui/material";
import React from "react";
import Tooltip from "@mui/material/Tooltip";

export const ProjectItem = (props: { ps: ProjectSummary }) => {
    const name = getLastPathElement(props.ps.project.gitRepoKey)
    const subDir = props.ps.project.subDir

    const projectInfo = <Box>
        {props.ps.project.gitRepoKey}<br/>
        {props.ps.project.subDir ? <>
            SubDir: {props.ps.project.subDir}<br/>
        </> : <></>}
    </Box>

    return <Paper elevation={5} sx={{width: "100%", height: "100%"}}>
        <Box display="flex" flexDirection={"column"} justifyContent={"space-between"} height={"100%"}>
            <Box m={1}>
                {name ?
                    <Tooltip title={projectInfo}>
                        <Typography
                            variant={"h6"}
                            textAlign={"center"}
                            textOverflow={"ellipsis"}
                            overflow={"hidden"}
                            whiteSpace={"nowrap"}
                        >
                            {name}
                        </Typography>
                    </Tooltip>
                    : <></>}
            </Box>
            <Box>
                {subDir ?
                    <Typography textAlign={"center"}>{subDir}</Typography>
                    : <></>}
            </Box>
            <Box marginTop={"auto"}>
                <Box display={"flex"}>
                    <Box marginLeft={"auto"}>
                        {/*<ActionsMenu menuItems={actionMenuItems}/>*/}
                    </Box>
                </Box>
            </Box>
        </Box>
    </Paper>
}

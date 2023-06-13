import { getLastPathElement } from "../../utils/misc";
import { Box, Typography } from "@mui/material";
import React from "react";
import Tooltip from "@mui/material/Tooltip";
import { ProjectIcon } from "../../icons/Icons";
import { ProjectSummary } from "../../project-summaries";
import { CardPaper } from "./Card";

export const ProjectItem = (props: { ps: ProjectSummary }) => {
    const name = getLastPathElement(props.ps.project.gitRepoKey)
    const subDir = props.ps.project.subDir

    const projectInfo = <Box>
        {props.ps.project.gitRepoKey}<br />
        {props.ps.project.subDir ? <>
            SubDir: {props.ps.project.subDir}<br />
        </> : <></>}
    </Box>

    return <CardPaper
        sx={{ padding: '5px 16px' }}
    >
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
                                flexGrow={1}
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
    </CardPaper>
}

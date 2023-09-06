import { getLastPathElement } from "../../utils/misc";
import { Box, Typography } from "@mui/material";
import React from "react";
import { ProjectIcon } from "../../icons/Icons";
import { ProjectSummary } from "../../project-summaries";
import { cardHeight, CardTemplate } from "../card/Card";

const projectCardWidth = 300

export const ProjectCard = React.memo((props: { ps: ProjectSummary }) => {
    let name = getLastPathElement(props.ps.project.gitRepoKey)
    const subDir = props.ps.project.subDir

    if (!name) {
        name = "<no-name>"
    }

    let projectInfo: React.ReactElement | undefined
    if (props.ps.project.gitRepoKey || props.ps.project.subDir) {
        projectInfo =  <Box>
            <Typography>{props.ps.project.gitRepoKey}</Typography>
            {props.ps.project.subDir && <Typography>SubDir: {props.ps.project.subDir}</Typography>}
        </Box>
    }

    return <CardTemplate
        paperProps={{
            sx: {
                padding: '20px 16px',
                width: projectCardWidth,
                height: cardHeight,
                minHeight: cardHeight
            }
        }}
        boxProps={{
            justifyContent: 'center'
        }}
        icon={name && <ProjectIcon />}
        header={name}
        headerTooltip={projectInfo}
        subheader={subDir}
    />;
});

ProjectCard.displayName = 'ProjectItem';

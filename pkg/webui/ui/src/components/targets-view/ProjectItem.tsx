import { getLastPathElement } from "../../utils/misc";
import { Box } from "@mui/material";
import React from "react";
import { ProjectIcon } from "../../icons/Icons";
import { ProjectSummary } from "../../project-summaries";
import { CardTemplate, cardWidth, projectCardMinHeight } from "./Card";

export const ProjectItem = React.memo((props: { ps: ProjectSummary }) => {
    const name = getLastPathElement(props.ps.project.gitRepoKey)
    const subDir = props.ps.project.subDir

    const projectInfo = <Box>
        {props.ps.project.gitRepoKey}<br />
        {props.ps.project.subDir ? <>
            SubDir: {props.ps.project.subDir}<br />
        </> : <></>}
    </Box>

    return <CardTemplate
        paperProps={{
            sx: {
                padding: '20px 16px',
                width: cardWidth,
                height: 'auto',
                minHeight: projectCardMinHeight
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

ProjectItem.displayName = 'ProjectItem';

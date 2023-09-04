import { Box } from "@mui/material";
import Tooltip from "@mui/material/Tooltip";
import React from "react";
import { Jdenticon } from "../Jdenticon";
import { TargetSummary } from "../../project-summaries";

export const DiscriminatorIcon = React.memo((props: { ts: TargetSummary }) => {
    const disabled = !props.ts.target.discriminator

    let tooltip = "Discriminator: " + props.ts.target.discriminator
    if (disabled) {
        tooltip = "Target has no discriminator"
    }

    return <Tooltip title={tooltip}>
        <Box>
            <Jdenticon value={props.ts.target.discriminator || ""} size={"48px"} disabled={disabled}/>
        </Box>
    </Tooltip>
})

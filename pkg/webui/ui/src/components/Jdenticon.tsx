import React, { useEffect, useRef } from 'react';
import * as jdenticon from 'jdenticon';
import { Box, SvgIcon } from "@mui/material";

export const Jdenticon = (props: { value: string, size: string }) => {
    const icon = useRef<SVGSVGElement>(null);
    useEffect(() => {
        if (icon.current === null) {
            return
        }
        jdenticon.update(icon.current, props.value);
    }, [props, icon])

    return <Box>
        <SvgIcon ref={icon} height={props.size} width={props.size}/>
    </Box>
}

import React, { useEffect, useRef } from 'react';
import * as jdenticon from 'jdenticon';
import { Box, SvgIcon } from "@mui/material";

export const Jdenticon = (props: { value: string, size: string, disabled?: boolean }) => {
    const icon = useRef<SVGSVGElement>(null);
    useEffect(() => {
        if (icon.current === null) {
            return
        }
        const sat = props.disabled ? 0.0 : 1.0
        jdenticon.update(icon.current, props.value, {saturation: {color: sat}});
    }, [props.value, props.size, props.disabled, icon])

    return <Box display={"flex"} alignItems={"center"}>
        <SvgIcon ref={icon} height={props.size} width={props.size}/>
    </Box>
}

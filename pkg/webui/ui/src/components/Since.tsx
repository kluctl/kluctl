import React, { useEffect, useState } from "react";
import { formatDurationShort } from "../utils/duration";
import Tooltip from "@mui/material/Tooltip";
import { Typography } from "@mui/material";

export const Since = (props: { startTime: Date | string }) => {
    const [str, setStr] = useState("")
    const [counter, setCounter] = useState(0)

    useEffect(() => {
        const update = () => {
            let st = props.startTime
            if (typeof st === "string") {
                st = new Date(st)
            }
            const d = new Date().getTime() - st.getTime()
            setStr(formatDurationShort(d))

            if (d > 60000) {
                return 10000
            } else {
                return 1000
            }
        }

        const t = update()
        const x = setTimeout(() => {
            setCounter(counter + 1)
        }, t)
        return () => clearTimeout(x)
    }, [props.startTime, counter])

    const tooltip = props.startTime.toString()

    return <Tooltip title={tooltip}>
        <Typography>{str}</Typography>
    </Tooltip>
}

import React from "react";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { CommandResult, CommandResultSummary } from "../../models";
import { Box, IconButton, Tooltip } from "@mui/material";
import { TreeViewIcon } from "../../icons/Icons";
import { Summarize } from "@mui/icons-material";
import { CardTemplate } from "../card/Card";
import { Since } from "../Since";
import { CommandResultSummaryBody } from "./CommandResultSummaryView";
import { useAppContext } from "../App";
import { CommandResultBody } from "./CommandResultView";
import { CommandResultStatusLine } from "./CommandResultStatusLine";
import { Loading, useLoadingHelper } from "../Loading";
import { ErrorMessage } from "../ErrorMessage";
import { CommandTypeIcon } from "../target-view/CommandTypeIcon";

export const CommandResultCard = React.memo(React.forwardRef((
    props: {
        current: boolean,
        ps: ProjectSummary,
        ts: TargetSummary,
        rs: CommandResultSummary,
        onSwitchFullCommandResult: () => void,
        showSummary: boolean,
        expanded: boolean,
        loadData: boolean,
        onClose?: () => void
    },
    ref: React.ForwardedRef<HTMLDivElement>
) => {
    const appCtx = useAppContext()

    const [loading, error, cr] = useLoadingHelper<CommandResult | undefined>(props.loadData, async () => {
        if (!props.loadData) {
            return undefined
        }
        return await appCtx.api.getCommandResult(props.rs.id)
    }, [props.rs.id, props.loadData])

    let header = props.rs.commandInfo?.command

    let body: React.ReactElement | undefined
    if (props.expanded && props.loadData) {
        if (loading) {
            body = <Box width={"100%"} height={"100%"}>
                <Loading />
            </Box>
        } else if (error) {
            body = <ErrorMessage>
                {error.message}
            </ErrorMessage>
        } else if (cr) {
            if (props.showSummary) {
                body = <CommandResultSummaryBody cr={cr}/>
            } else {
                body = <CommandResultBody cr={cr}/>
            }
        }
    }

    const footer = <>
        <Box display='flex' gap='6px' alignItems='center' flex={"1 1 auto"}>
            <CommandResultStatusLine rs={props.rs} />
        </Box>
        <Box display='flex' gap='6px' alignItems='center' height='39px'>
            <IconButton
                onClick={e => {
                    e.stopPropagation();
                    props.onSwitchFullCommandResult()
                }}
                sx={{
                    padding: 0,
                    width: 26,
                    height: 26
                }}
            >
                <Tooltip title={props.showSummary ? "Show full result tree" : "Show summary"}>
                    <Box display='flex'>{props.showSummary ? <TreeViewIcon /> : <Summarize/>}</Box>
                </Tooltip>
            </IconButton>
        </Box>
    </>

    return <CardTemplate
        ref={ref}
        showCloseButton={props.expanded}
        onClose={props.onClose}
        paperProps={{
            sx: {
                padding: '20px 16px 5px 16px',
            },
        }}
        icon={<CommandTypeIcon ts={props.ts} rs={props.rs}/>}
        header={header}
        subheader={<Since startTime={new Date(props.rs.commandInfo.startTime)}/>}
        subheaderTooltip={props.rs.commandInfo.startTime}
        body={body}
        footer={footer}
    />;
}));
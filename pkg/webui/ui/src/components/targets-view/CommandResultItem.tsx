import React, { useContext, useEffect, useMemo, useState } from "react";
import { CommandResultSummary, ShortName } from "../../models";
import * as yaml from "js-yaml";
import { CodeViewer } from "../CodeViewer";
import { Box, IconButton, SxProps, Theme, Tooltip } from "@mui/material";
import { CommandResultStatusLine } from "../result-view/CommandResultStatusLine";
import { useNavigate } from "react-router";
import { DeployIcon, DiffIcon, PruneIcon, TreeViewIcon } from "../../icons/Icons";
import { calcAgo } from "../../utils/duration";
import { CardBody, CardTemplate, cardHeight, cardWidth } from "./Card";
import { ApiContext, AppContext } from "../App";
import { Loading } from "../Loading";
import { NodeBuilder } from "../result-view/nodes/NodeBuilder";
import { ErrorMessage } from "../ErrorMessage";
import { Api } from "../../api";
import { CommandResultNodeData } from "../result-view/nodes/CommandResultNode";

async function doGetRootNode(api: Api, rs: CommandResultSummary, shortNames: ShortName[]) {
    const r = await api.getCommandResult(rs.id);
    const builder = new NodeBuilder({
        shortNames,
        summary: rs,
        commandResult: r,
    });
    const [node] = builder.buildRoot();
    return node;
}

export const CommandResultItemBody = React.memo((props: {
    rs: CommandResultSummary,
    loadData?: boolean
}) => {
    const api = useContext(ApiContext);
    const appContext = useContext(AppContext);

    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<any>();
    const [node, setNode] = useState<CommandResultNodeData>();
    const [prevRsId, setPrevRsId] = useState<string>();

    useEffect(() => {
        let cancelled = false;
        if (!props.loadData) {
            return;
        }

        if (prevRsId === props.rs.id) {
            return;
        }

        const doStartLoading = async () => {
            try {
                setLoading(true);
                const n = await doGetRootNode(api, props.rs, appContext.shortNames);
                if (cancelled) {
                    return;
                }
                setNode(n);
                setPrevRsId(props.rs.id);
            } catch (error) {
                setError(error);
            }
            setLoading(false);
        };

        doStartLoading();

        return () => {
            cancelled = true;
        }
    }, [api, appContext.shortNames, prevRsId, props.loadData, props.rs])

    if (loading) {
        return <Loading />;
    }

    if (error) {
        return <ErrorMessage>
            {error.message}
        </ErrorMessage>;
    }

    if (!node) {
        return null;
    }

    return <CardBody provider={node} />
});

export const CommandResultItem = React.memo(React.forwardRef((
    props: {
        rs: CommandResultSummary,
        onSelectCommandResult?: (rs: CommandResultSummary) => void,
        sx?: SxProps<Theme>,
        expanded?: boolean,
        loadData?: boolean,
        onClose?: () => void
    },
    ref: React.ForwardedRef<HTMLDivElement>
) => {
    const {
        rs,
        onSelectCommandResult,
        sx,
        expanded,
        loadData,
        onClose
    } = props;
    const navigate = useNavigate();
    const [ago, setAgo] = useState(calcAgo(rs.commandInfo.startTime))

    let Icon: () => JSX.Element = DiffIcon
    switch (rs.commandInfo?.command) {
        case "delete":
            Icon = PruneIcon
            break
        case "deploy":
            Icon = DeployIcon
            break
        case "diff":
            Icon = DiffIcon
            break
        case "poke-images":
            Icon = DeployIcon
            break
        case "prune":
            Icon = PruneIcon
            break
    }

    const iconTooltip = useMemo(() => {
        const cmdInfoYaml = yaml.dump(rs.commandInfo);
        return <CodeViewer code={cmdInfoYaml} language={"yaml"} />
    }, [rs.commandInfo]);

    useEffect(() => {
        const interval = setInterval(() => setAgo(calcAgo(rs.commandInfo.startTime)), 5000);
        return () => clearInterval(interval);
    }, [rs.commandInfo.startTime]);

    const body = expanded ? <CommandResultItemBody rs={rs} loadData={loadData} /> : undefined;

    return <CardTemplate
        ref={ref}
        showCloseButton={expanded}
        onClose={onClose}
        paperProps={{
            sx: {
                padding: '20px 16px 5px 16px',
                width: cardWidth,
                height: cardHeight,
                ...sx,
            },
            onClick: () => onSelectCommandResult?.(rs)
        }}
        icon={<Icon />}
        iconTooltip={iconTooltip}
        header={rs.commandInfo?.command}
        subheader={ago}
        subheaderTooltip={rs.commandInfo.startTime}
        body={body}
        footer={
            <>
                <Box display='flex' gap='6px' alignItems='center'>
                    <CommandResultStatusLine rs={rs} />
                </Box>
                <Box display='flex' gap='6px' alignItems='center' height='39px'>
                    <IconButton
                        onClick={e => {
                            e.stopPropagation();
                            navigate(`/results/${rs.id}`);
                        }}
                        sx={{
                            padding: 0,
                            width: 26,
                            height: 26
                        }}
                    >
                        <Tooltip title={"Open Result Tree"}>
                            <Box display='flex'><TreeViewIcon /></Box>
                        </Tooltip>
                    </IconButton>
                </Box>
            </>
        }
    />;
}));

CommandResultItem.displayName = 'CommandResultItem';

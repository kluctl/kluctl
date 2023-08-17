import React, { useContext, useEffect, useMemo, useState } from "react";
import { CommandResultSummary, ShortName } from "../../models";
import * as yaml from "js-yaml";
import { CodeViewer } from "../CodeViewer";
import { Box, IconButton, SxProps, Theme, Tooltip } from "@mui/material";
import { CommandResultStatusLine } from "../result-view/CommandResultStatusLine";
import { useNavigate } from "react-router";
import { DeployIcon, DiffIcon, PruneIcon, TreeViewIcon } from "../../icons/Icons";
import { CardBody, CardTemplate } from "./Card";
import { ApiContext, AppContext, UserContext } from "../App";
import { Loading } from "../Loading";
import { NodeBuilder } from "../result-view/nodes/NodeBuilder";
import { ErrorMessage } from "../ErrorMessage";
import { Api } from "../../api";
import { CommandResultNodeData } from "../result-view/nodes/CommandResultNode";
import { Since } from "../Since";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { LiveHelp, RocketLaunch } from "@mui/icons-material";

async function doGetRootNode(api: Api, rs: CommandResultSummary, shortNames: ShortName[]) {
    const r = await api.getCommandResult(rs.id);
    const builder = new NodeBuilder({
        shortNames,
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

const ApprovalIcon = (props: {ts: TargetSummary, rs: CommandResultSummary}) => {
    const api = useContext(ApiContext)
    const user = useContext(UserContext)
    const handleApprove = (approve: boolean) => {
        if (!props.ts.kdInfo || !props.ts.kd) {
            return
        }
        if (approve) {
            if (!props.rs.renderedObjectsHash) {
                return
            }
            api.setManualObjectsHash(props.ts.kdInfo.clusterId, props.ts.kdInfo.name, props.ts.kdInfo.namespace, props.rs.renderedObjectsHash)
        } else {
            api.setManualObjectsHash(props.ts.kdInfo.clusterId, props.ts.kdInfo.name, props.ts.kdInfo.namespace, "")
        }
    }

    if (!user?.isAdmin || props.ts.kd?.deployment.spec.dryRun || !props.ts.kd?.deployment.spec.manual) {
        return <></>
    }
    if (props.rs.id !== props.ts.commandResults[0].id) {
        return <></>
    }

    if (!props.rs.commandInfo.dryRun || !props.rs.renderedObjectsHash) {
        return <></>
    }

    const isApproved = props.ts.kd.deployment.spec.manualObjectsHash === props.rs.renderedObjectsHash

    let icon: React.ReactElement
    let tooltip: string
    if (!isApproved) {
        tooltip = "Click here to trigger this manual deployment."
        icon = <RocketLaunch color={"info"}/>
    } else {
        tooltip = "Click here to cancel this deployment. This will only have an effect if the deployment has not started reconciliation yet!"
        icon = <RocketLaunch color={"success"}/>
    }
    return <Box display='flex' gap='6px' alignItems='center' height='39px'>
        <IconButton
            onClick={e => {
                e.stopPropagation();
                handleApprove(!isApproved)
            }}
            sx={{
                padding: 0,
                width: 26,
                height: 26
            }}
        >
            <Tooltip title={tooltip}>
                <Box display='flex'>{icon}</Box>
            </Tooltip>
        </IconButton>
    </Box>
}

export const CommandResultItem = React.memo(React.forwardRef((
    props: {
        current: boolean,
        ps: ProjectSummary,
        ts: TargetSummary,
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

    let icon: React.ReactElement
    let cardGlow = false
    let header = rs.commandInfo?.command
    switch (rs.commandInfo?.command) {
        default:
            icon = <DiffIcon/>
            break
        case "delete":
            icon = <PruneIcon/>
            break
        case "deploy":
            if (rs.commandInfo.dryRun) {
                if (props.ts.kd?.deployment.spec.manual && !props.ts.kd?.deployment.spec.dryRun) {
                    icon = <LiveHelp sx={{ width: "100%", height: "100%" }}/>
                    cardGlow = true
                    header = "manual deploy"
                } else {
                    icon = <DeployIcon/>
                    header = "dry-run deploy"
                }
            } else {
                icon = <DeployIcon/>
            }
            break
        case "diff":
            icon = <DiffIcon/>
            break
        case "poke-images":
            icon = <DeployIcon/>
            break
        case "prune":
            icon = <PruneIcon/>
            break
    }

    const iconTooltip = useMemo(() => {
        const cmdInfoYaml = yaml.dump(rs.commandInfo);
        return <CodeViewer code={cmdInfoYaml} language={"yaml"} />
    }, [rs.commandInfo]);

    const body = expanded ? <CommandResultItemBody rs={rs} loadData={loadData} /> : undefined;

    return <CardTemplate
        ref={ref}
        showCloseButton={expanded}
        onClose={onClose}
        paperProps={{
            sx: {
                padding: '20px 16px 5px 16px',
                ...sx,
            },
            glow: cardGlow,
            onClick: () => onSelectCommandResult?.(rs)
        }}
        icon={icon}
        iconTooltip={iconTooltip}
        header={header}
        subheader={<Since startTime={new Date(rs.commandInfo.startTime)}/>}
        subheaderTooltip={rs.commandInfo.startTime}
        body={body}
        footer={
            <>
                <Box display='flex' gap='6px' alignItems='center' flex={"1 1 auto"}>
                    <CommandResultStatusLine rs={rs} />
                </Box>
                <ApprovalIcon ts={props.ts} rs={props.rs}/>
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

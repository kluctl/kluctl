import { ValidateResult } from "../../models";
import { ActionMenuItem, ActionsMenu } from "../ActionsMenu";
import { Box, SxProps, Theme, Typography, useTheme } from "@mui/material";
import React, { useContext } from "react";
import Tooltip from "@mui/material/Tooltip";
import { Favorite, HeartBroken, PublishedWithChanges } from "@mui/icons-material";
import { CpuIcon, FingerScanIcon, MessageQuestionIcon, TargetIcon } from "../../icons/Icons";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { calcAgo } from "../../utils/duration";
import { ApiContext } from "../App";
import { CardBody, cardHeight, CardTemplate, cardWidth } from "./Card";
import { SidePanelProvider, SidePanelTab } from "../result-view/SidePanel";
import { ErrorsTable } from "../ErrorsTable";
import { PropertiesTable } from "../PropertiesTable";
import { Loading, useLoadingHelper } from "../Loading";
import { ErrorMessage } from "../ErrorMessage";

const StatusIcon = (props: { ps: ProjectSummary, ts: TargetSummary }) => {
    let icon: React.ReactElement
    const theme = useTheme();

    let validateEnabled = false
    props.ts.kluctlDeployments.forEach(kd => {
        if (kd.deployment.spec.validate) {
            validateEnabled = true
        }
    })

    if (!validateEnabled) {
        icon = <Favorite color={"disabled"}/>
    } else if (props.ts.lastValidateResult === undefined) {
        icon = <MessageQuestionIcon color={theme.palette.error.main} />
    } else if (props.ts.lastValidateResult.ready && !props.ts.lastValidateResult.errors) {
        if (props.ts.lastValidateResult.warnings) {
            icon = <Favorite color={"warning"} />
        /*} else if (props.ts.lastValidateResult.drift?.length) {
            icon = <Favorite color={"primary"} />*/
        } else {
            icon = <Favorite color={"success"} />
        }
    } else {
        icon = <HeartBroken color={"error"} />
    }

    const tooltip: string[] = []
    if (!validateEnabled) {
        tooltip.push("Validation is disabled.")
    } else if (props.ts.lastValidateResult === undefined) {
        tooltip.push("No validation result available.")
    } else {
        if (props.ts.lastValidateResult.ready && !props.ts.lastValidateResult.errors) {
            tooltip.push("Target is ready.")
        } else {
            tooltip.push("Target is not ready.")
        }
        if (props.ts.lastValidateResult.errors) {
            tooltip.push(`Target has ${props.ts.lastValidateResult.errors} validation errors.`)
        }
        if (props.ts.lastValidateResult.warnings) {
            tooltip.push(`Target has ${props.ts.lastValidateResult.warnings} validation warnings.`)
        }
        /*if (props.ts.lastValidateResult.drift?.length) {
            tooltip.push(`Target has ${props.ts.lastValidateResult.drift.length} drifted objects.`)
        }*/
        tooltip.push("Validation performed " + calcAgo(props.ts.lastValidateResult.startTime) + " ago")
    }

    return <Tooltip title={tooltip.map(t => <Typography key={t}>{t}</Typography>)}>
        <Box display='flex'>{icon}</Box>
    </Tooltip>
}

export const TargetItemBody = React.memo((props: {
    ts: TargetSummary
}) => {
    const api = useContext(ApiContext);

    const [loading, error, vr] = useLoadingHelper<ValidateResult | undefined>(async () => {
        if (!props.ts.lastValidateResult) {
            return undefined
        }
        const o = await api.getValidateResult(props.ts.lastValidateResult?.id)
        return o
    }, [props.ts.lastValidateResult?.id])

    if (loading) {
        return <Loading />;
    }

    if (error) {
        return <ErrorMessage>
            {error.message}
        </ErrorMessage>;
    }

    return <CardBody provider={new MyProvider(props.ts, vr)}/>
});

export const TargetItem = React.memo(React.forwardRef((
    props: {
        ps: ProjectSummary,
        ts: TargetSummary,
        onSelectTarget?: (ts: TargetSummary) => void,
        sx?: SxProps<Theme>,
        expanded?: boolean,
        onClose?: () => void
    },
    ref: React.ForwardedRef<HTMLDivElement>
) => {
    const api = useContext(ApiContext)
    const actionMenuItems: ActionMenuItem[] = []

    props.ts.kluctlDeployments.forEach(kd => {
        actionMenuItems.push({
            icon: <PublishedWithChanges />,
            text: <Typography>Validate <b>{kd.deployment.metadata.name}</b></Typography>,
            handler: () => {
                api.validateNow(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace)
            }
        })
        actionMenuItems.push({
            icon: <PublishedWithChanges />,
            text: <Typography>Reconcile <b>{kd.deployment.metadata.name}</b></Typography>,
            handler: () => {
                api.reconcileNow(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace)
            }
        })
        actionMenuItems.push({
            icon: <PublishedWithChanges />,
            text: <Typography>Deploy <b>{kd.deployment.metadata.name}</b></Typography>,
            handler: () => {
                api.deployNow(kd.clusterId, kd.deployment.metadata.name, kd.deployment.metadata.namespace)
            }
        })
    })

    const allContexts: string[] = []

    const handleContext = (c?: string) => {
        if (!c) {
            return
        }
        if (allContexts.find(x => x === c)) {
            return
        }
        allContexts.push(c)
    }

    props.ts.commandResults?.forEach(rs => {
        handleContext(rs.commandInfo.contextOverride)
        handleContext(rs.target.context)
    })

    let header = "<command-line>"
    if (props.ts.kluctlDeployments.length) {
        header = props.ts.kluctlDeployments[0].deployment.metadata.name
    }

    let subheader = "<no-name>"
    if (props.ts.target.targetName) {
        subheader = props.ts.target.targetName
    }

    const iconTooltipChildren: React.ReactNode[] = []
    if (props.ts.kluctlDeployments.length) {
        iconTooltipChildren.push(<Typography variant={"subtitle2"}><b>KluctlDeployments</b></Typography>)
        props.ts.kluctlDeployments.forEach(kd => {
            iconTooltipChildren.push(<Typography variant={"subtitle2"}>{kd.deployment.metadata.name}</Typography>)
        })
        iconTooltipChildren.push(<br/>)
    }
    if (allContexts.length) {
        iconTooltipChildren.push(<Typography variant="subtitle2"><b>Contexts</b></Typography>)
        allContexts.forEach(context => {
            iconTooltipChildren.push(<Typography key={context} variant="subtitle2">{context}</Typography>)
        })
    }
    const iconTooltip = <Box textAlign={"center"}>{iconTooltipChildren}</Box>

    const body = props.expanded ? <TargetItemBody ts={props.ts} /> : undefined;

    return <CardTemplate
        ref={ref}
        showCloseButton={props.expanded}
        onClose={props.onClose}
        paperProps={{
            sx: {
                padding: '20px 16px 12px 16px',
                width: cardWidth,
                height: cardHeight,
                ...props.sx
            },
            onClick: e => props.onSelectTarget?.(props.ts)
        }}
        icon={<TargetIcon />}
        iconTooltip={iconTooltip}
        header={header}
        subheader={subheader}
        body={body}
        footer={
            <>
                <Box display='flex' gap='6px' alignItems='center'>
                    <Tooltip title={"Cluster ID: " + props.ts.target.clusterId}>
                        <Box display='flex'><CpuIcon /></Box>
                    </Tooltip>
                    <Tooltip title={"Discriminator: " + props.ts.target.discriminator}>
                        <Box display='flex'><FingerScanIcon /></Box>
                    </Tooltip>
                </Box>
                <Box display='flex' gap='6px' alignItems='center'>
                    <StatusIcon {...props} />
                    <ActionsMenu menuItems={actionMenuItems} />
                </Box>
            </>
        }
    />;
}));

TargetItem.displayName = 'TargetItem';

class MyProvider implements SidePanelProvider {
    private ts?: TargetSummary;
    private lastValidateResult?: ValidateResult

    constructor(ts?: TargetSummary, vr?: ValidateResult) {
        this.ts = ts
        this.lastValidateResult = vr
    }

    buildSidePanelTabs(): SidePanelTab[] {
        if (!this.ts) {
            return []
        }

        const tabs = [
            { label: "Summary", content: this.buildSummaryTab() }
        ]

        if (this.lastValidateResult?.errors) {
            tabs.push({
                label: "Errors",
                content: <ErrorsTable errors={this.lastValidateResult.errors} />
            })
        }
        if (this.lastValidateResult?.warnings) {
            tabs.push({
                label: "Warnings",
                content: <ErrorsTable errors={this.lastValidateResult.warnings} />
            })
        }

        return tabs
    }

    buildSummaryTab(): React.ReactNode {
        const props = [
            { name: "Target Name", value: this.getTargetName() },
            { name: "Discriminator", value: this.ts?.target.discriminator },
        ]

        if (this.ts?.lastValidateResult) {
            props.push({ name: "Ready", value: this.ts?.lastValidateResult.ready + "" })
        }
        if (this.ts?.lastValidateResult?.errors) {
            props.push({ name: "Errors", value: this.ts?.lastValidateResult.errors + "" })
        }
        if (this.ts?.lastValidateResult?.warnings) {
            props.push({ name: "Warnings", value: this.ts?.lastValidateResult.warnings + "" })
        }

        return <>
            <PropertiesTable properties={props} />
        </>
    }

    getTargetName() {
        if (!this.ts) {
            return ""
        }

        let name = "<no-name>"
        if (this.ts.target.targetName) {
            name = this.ts.target.targetName
        }
        return name
    }

    buildSidePanelTitle(): React.ReactNode {
        return this.getTargetName()
    }
}

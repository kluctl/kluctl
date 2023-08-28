import { ValidateResult } from "../../models";
import { Alert, Box, SxProps, Theme, Typography } from "@mui/material";
import React, { useState } from "react";
import Tooltip from "@mui/material/Tooltip";
import { CpuIcon, FingerScanIcon, TargetIcon } from "../../icons/Icons";
import { ProjectSummary, TargetSummary } from "../../project-summaries";
import { CardBody, CardTemplate } from "../card/Card";
import { CardTab, CardTabsProvider } from "../card/CardTabs";
import { ErrorsTable } from "../ErrorsTable";
import { PropertiesEntry, PropertiesTable, pushProp } from "../PropertiesTable";
import { Loading, useLoadingHelper } from "../Loading";
import { ErrorMessage } from "../ErrorMessage";
import { ValidateResultsTable } from "../ValidateResultsTable";
import { LogsViewer } from "../LogsViewer";
import { K8sManifestViewer } from "../K8sManifestViewer";
import { YamlViewer } from "../YamlViewer";
import { gitRefToString } from "../../utils/git";
import { AppContextProps, useAppContext } from "../App";
import { ReconcilingIcon } from "../targets-view/ReconcilingIcon";
import { StatusIcon } from "../targets-view/StatusIcon";
import { TargetTypeIcon } from "../targets-view/TargetTypeIcon";
import { TargetActionMenu } from "../targets-view/TargetActionMenu";

export const TargetItemBody = React.memo((props: {
    ts: TargetSummary
}) => {
    const appCtx = useAppContext()
    const [initialLoading, setInitialLoading] = useState(true)

    const [loading, error, vr] = useLoadingHelper<ValidateResult | undefined>(true, async () => {
        if (!props.ts.lastValidateResult) {
            return undefined
        }
        const o = await appCtx.api.getValidateResult(props.ts.lastValidateResult?.id)
        return o
    }, [props.ts.lastValidateResult?.id])

    if (initialLoading && loading) {
        return <Loading/>;
    } else if (initialLoading) {
        setInitialLoading(false)
    }

    if (error) {
        return <ErrorMessage>
            {error.message}
        </ErrorMessage>;
    }

    return <CardBody provider={new TargetItemCardProvider(props.ts, vr)}/>
});

export const TargetCard = React.memo(React.forwardRef((
    props: {
        ps: ProjectSummary,
        ts: TargetSummary,
        onSelectTarget?: () => void,
        sx?: SxProps<Theme>,
        expanded?: boolean,
        onClose?: () => void
    },
    ref: React.ForwardedRef<HTMLDivElement>
) => {
    const kd = props.ts.kd

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
    if (kd) {
        header = kd.deployment.metadata.name
    }

    let subheader = "<no-name>"
    if (props.ts.target.targetName) {
        subheader = props.ts.target.targetName
    }

    const iconTooltipChildren: React.ReactNode[] = []
    if (kd) {
        iconTooltipChildren.push(<Typography key={iconTooltipChildren.length} variant={"subtitle2"}><b>KluctlDeployments</b></Typography>)
        iconTooltipChildren.push(<Typography key={iconTooltipChildren.length} variant={"subtitle2"}>{kd.deployment.metadata.name}</Typography>)
        iconTooltipChildren.push(<br key={iconTooltipChildren.length}/>)
    }
    if (allContexts.length) {
        iconTooltipChildren.push(<Typography key={iconTooltipChildren.length} variant="subtitle2"><b>Contexts</b></Typography>)
        allContexts.forEach(context => {
            iconTooltipChildren.push(<Typography key={iconTooltipChildren.length} variant="subtitle2">{context}</Typography>)
        })
    }
    const iconTooltip = <Box textAlign={"center"}>{iconTooltipChildren}</Box>

    const body = props.expanded ? <TargetItemBody ts={props.ts}/> : undefined;

    return <CardTemplate
        ref={ref}
        showCloseButton={props.expanded}
        onClose={props.onClose}
        paperProps={{
            sx: {
                padding: '20px 16px 12px 16px',
                ...props.sx
            },
            onClick: e => props.onSelectTarget?.()
        }}
        icon={<TargetIcon/>}
        iconTooltip={iconTooltip}
        header={header}
        subheader={subheader}
        body={body}
        footer={
            <>
                <Box display='flex' gap='6px' alignItems='center'>
                    <Tooltip title={"Cluster ID: " + props.ts.target.clusterId}>
                        <Box display='flex'><CpuIcon/></Box>
                    </Tooltip>
                    <Tooltip title={"Discriminator: " + props.ts.target.discriminator}>
                        <Box display='flex'><FingerScanIcon/></Box>
                    </Tooltip>
                    <TargetTypeIcon ts={props.ts}/>
                </Box>
                <Box display='flex' gap='6px' alignItems='center'>
                    <ReconcilingIcon {...props} />
                    <StatusIcon {...props} />
                    <TargetActionMenu ts={props.ts}/>
                </Box>
            </>
        }
    />;
}));

TargetCard.displayName = 'TargetItem';

class TargetItemCardProvider implements CardTabsProvider {
    private ts?: TargetSummary;
    private lastValidateResult?: ValidateResult

    constructor(ts?: TargetSummary, vr?: ValidateResult) {
        this.ts = ts
        this.lastValidateResult = vr
    }

    buildSidePanelTabs(appCtx: AppContextProps): CardTab[] {
        if (!this.ts) {
            return []
        }

        const tabs = [
            { label: "Summary", content: this.buildSummaryTab() }
        ]

        if (this.ts.kd?.deployment) {
            tabs.push({
                label: "KluctlDeployment",
                content: <K8sManifestViewer obj={this.ts.kd.deployment} initialShowStatus={false}/>
            })
        }

        if (this.lastValidateResult?.results) {
            tabs.push({
                label: "Validation Results",
                content: <ValidateResultsTable results={this.lastValidateResult.results}/>
            })
        }
        if (this.lastValidateResult?.errors) {
            tabs.push({
                label: "Validation Errors",
                content: <ErrorsTable errors={this.lastValidateResult.errors}/>
            })
        }
        if (this.lastValidateResult?.warnings) {
            tabs.push({
                label: "Validation Warnings",
                content: <ErrorsTable errors={this.lastValidateResult.warnings}/>
            })
        }

        if (!appCtx.isStatic && this.ts.kd) {
            tabs.push({
                label: "Logs", content: <LogsViewer
                    cluster={this.ts.kdInfo?.clusterId}
                    name={this.ts.kdInfo?.name}
                    namespace={this.ts.kdInfo?.namespace}
                />
            })
        }

        return tabs
    }

    buildSummaryTab(): React.ReactNode {
        const d = this.ts?.kd?.deployment

        const props: PropertiesEntry[] = [
            { name: "Target Name", value: this.getTargetName() },
        ]

        pushProp(props, "Discriminator", this.ts?.target.discriminator)

        if (d) {
            let args = d.spec.args
            if (args && Object.keys(args).length === 0) {
                args = undefined
            }

            pushProp(props, "Interval", d.spec.interval)
            pushProp(props, "Retry Interval", d.spec.retryInterval)
            pushProp(props, "Deploy Interval", d.spec.deployInterval)
            pushProp(props, "Validate Interval", d.spec.validateInterval)
            pushProp(props, "Timeout", d.spec.timeout)
            pushProp(props, "Suspend", d.spec.suspend)
            pushProp(props, "Target", d.spec.target)
            pushProp(props, "Target Name Override", d.spec.targetNameOverride)
            pushProp(props, "Context", d.spec.context)
            pushProp(props, "Args", args, () => <YamlViewer obj={args}/>)
            pushProp(props, "Dry Run", d.spec.dryRun)
            pushProp(props, "No Wait", d.spec.noWait)
            pushProp(props, "Force Apply", d.spec.forceApply)
            pushProp(props, "Replace On Error", d.spec.replaceOnError)
            pushProp(props, "Force Replace On Error", d.spec.forceReplaceOnError)
            pushProp(props, "Abort On Error", d.spec.abortOnError)
            pushProp(props, "Include Tags", d.spec.includeTags)
            pushProp(props, "Exclude Tags", d.spec.excludeTags)
            pushProp(props, "Include Deployment Dirs", d.spec.includeDeploymentDirs)
            pushProp(props, "Exclude Deployment Dirs", d.spec.excludeDeploymentDirs)
            pushProp(props, "Deploy Mode", d.spec.deployMode)
            pushProp(props, "Validate", d.spec.validate)
            pushProp(props, "Prune", d.spec.prune)
            pushProp(props, "Delete", d.spec.delete)
            pushProp(props, "Manual", d.spec.manual)
            pushProp(props, "Manual Objects Hash", d.spec.manualObjectsHash)

            pushProp(props, "Source Url", d.spec.source.url)
            pushProp(props, "Source Ref", gitRefToString(d.spec.source.ref))
            pushProp(props, "Source Path", d.spec.source.path)

            pushProp(props, "Last Objects Hash", d.status.lastObjectsHash)
        }

        pushProp(props, "Ready", this.ts?.lastValidateResult?.ready)
        pushProp(props, "Errors", this.ts?.lastValidateResult?.errors)
        pushProp(props, "Warnings", this.ts?.lastValidateResult?.warnings)

        let errorHeader
        if (d?.status) {
            if (d.status.lastPrepareError) {
                errorHeader = <Alert severity="error">
                    The prepare step failed for this deployment. This usually means that your deployment is severely
                    broken and can't even be loaded.<br/>
                    The error message is: <b>{d.status.lastPrepareError}</b>
                </Alert>
            }
        }

        return <Box flex={"1 1 auto"}>
            {errorHeader}
            <PropertiesTable properties={props}/>
        </Box>
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

import { buildTargetKey, ProjectSummary, TargetSummary } from "../../project-summaries";
import { CommandResultSummary } from "../../models";
import { useAppContext } from "../App";
import React, { useCallback, useMemo } from "react";
import { Box, Dialog, IconButton, Typography } from "@mui/material";
import { DataGrid, GridColDef, gridStringOrNumberComparator } from '@mui/x-data-grid';
import { getLastPathElement } from "../../utils/misc";
import Tooltip from "@mui/material/Tooltip";
import { Approval, Keyboard, Toys } from "@mui/icons-material";
import { ReconcilingIcon } from "../target-view/ReconcilingIcon";
import { StatusIcon } from "../target-view/StatusIcon";
import { TargetActionMenu } from "../target-view/TargetActionMenu";
import { CommandResultStatusLine, DriftDetectionResultStatusLine } from "../command-result/CommandResultStatusLine";
import { Since } from "../Since";
import { ScrollingTextLine } from "../ScrollingTextLine";
import { ClusterIcon } from "../target-view/ClusterIcon";
import { TargetCard } from "../target-cards-view/TargetCard";
import { CommandResultCard } from "../command-result/CommandResultCard";
import { ManualApproveButton } from "../target-view/ManualApproveButton";
import { TreeViewIcon } from "../../icons/Icons";


export interface TargetsListViewProps {
    selectedProject?: ProjectSummary
    selectedTarget?: TargetSummary
    selectedResult?: CommandResultSummary
    selectedResultFull?: boolean
    onSelect: (ps: ProjectSummary, ts: TargetSummary, showResults: boolean, rs?: CommandResultSummary, full?: boolean) => void
    onCloseExpanded: () => void
}

export const TargetsListView = (props: TargetsListViewProps) => {
    const appContext = useAppContext();
    const projects = appContext.projects;

    interface Row {
        id: string,
        ps: ProjectSummary
        ts: TargetSummary
    }

    const onSelect = props.onSelect
    const doSelect = useCallback((ps: ProjectSummary, ts: TargetSummary, showResults: boolean, rs?: CommandResultSummary, full?: boolean) => {
        console.log("select", ps, ts, showResults, rs)
        onSelect(ps, ts, showResults, rs, full)
    }, [onSelect])

    const doSelectTarget = (row: Row) => {
        doSelect(row.ps, row.ts, false)
    }
    const doSelectCommandResult = (row: Row, index: number, full?: boolean) => {
        if (index >= (row.ts.commandResults.length || 0)) {
            return
        }
        const rs = row.ts.commandResults[index]
        doSelect(row.ps, row.ts, true, rs, full)
    }

    const columns: GridColDef<Row>[] = [
        {
            field: "cli",
            headerName: "CLI",
            width: 48,
            valueGetter: rp => {
                return !rp.row.ts.kdInfo
            },
            renderCell: rp => {
                if (rp.value) {
                    const tooltip = <Typography>Invoked via the CLI</Typography>
                    return <Tooltip title={tooltip}>
                        <Keyboard/>
                    </Tooltip>
                } else {
                    return <></>
                }
            }
        },
        {
            field: "project",
            headerName: "Project",
            width: 200,
            valueGetter: rp => {
                return rp.row.ps.project.repoKey
            },
            sortComparator: (v1, v2, cellParams1, cellParams2) => {
                const name1 = getLastPathElement(v1)
                const name2 = getLastPathElement(v2)
                if (name1 === name2) {
                    return gridStringOrNumberComparator(v1, v2, cellParams1, cellParams2)
                }
                return gridStringOrNumberComparator(name1, name2, cellParams1, cellParams2)
            },
            renderCell: (rp) => {
                const name = getLastPathElement(rp.value)
                return <Box width={"100%"}>
                    <Tooltip title={<Typography>{rp.value}</Typography>}>
                        <ScrollingTextLine>
                            {name}
                        </ScrollingTextLine>
                    </Tooltip>
                </Box>
            }
        },
        {
            field: "subDir",
            headerName: "SubDir",
            valueGetter: rp => {
                return rp.row.ps.project.subDir
            },
            renderCell: rp => {
                return <ScrollingTextLine>{rp.value}</ScrollingTextLine>
            }
        },
        {
            field: "cluster",
            headerName: "Cluster",
            valueGetter: rp => {
                return rp.row.ts.target.clusterId
            },
            renderCell: rp => {
                return <ClusterIcon ts={rp.row.ts}/>
            }
        },
        {
            field: "targetName",
            headerName: "Target",
            width: 100,
            valueGetter: rp => {
                return rp.row.ts.target.targetName
            },
            renderCell: rp => {
                return <Box width={"100%"} onDoubleClick={() => doSelectTarget(rp.row)}>
                    <ScrollingTextLine>{rp.value}</ScrollingTextLine>
                </Box>
            }
        },
        {
            field: "discriminator",
            headerName: "Discriminator",
            width: 200,
            valueGetter: rp => {
                return rp.row.ts.target.discriminator
            },
            renderCell: rp => {
                return <Box width={"100%"} onDoubleClick={() => doSelectTarget(rp.row)}>
                    <ScrollingTextLine>{rp.value}</ScrollingTextLine>
                </Box>
            }
        },
        {
            field: "type",
            headerName: "Type",
            width: 48,
            valueGetter: rp => {
                if (rp.row.ts.kd?.deployment.spec.manual) {
                    return "manual"
                } else if (rp.row.ts.kd?.deployment.spec.dryRun) {
                    return "dryRun"
                } else {
                    return "normal"
                }
            },
            renderCell: rp => {
                if (rp.value === "manual") {
                    return <Tooltip title={"Deployment needs manual triggering"}>
                        <Approval/>
                    </Tooltip>
                } else if (rp.value === "dryRun") {
                    return <Tooltip title={"Deployment needs manual triggering"}>
                        <Toys/>
                    </Tooltip>
                } else {
                    return <></>
                }
            }
        },
        {
            field: "reconcileStatus",
            headerName: "Rec",
            description: "Reconcile Status",
            sortable: false,
            filterable: false,
            width: 48,
            renderCell: rp => {
                return <Box onDoubleClick={() => doSelectTarget(rp.row)}>
                    <ReconcilingIcon ps={rp.row.ps} ts={rp.row.ts}/>
                </Box>
            }
        },
        {
            field: "status",
            headerName: "Status",
            width: 48,
            sortable: false,
            filterable: false,
            renderCell: rp => {
                return <Box onDoubleClick={() => doSelectTarget(rp.row)}>
                    <StatusIcon ps={rp.row.ps} ts={rp.row.ts}/>
                </Box>
            }
        },
        {
            field: "drift",
            headerName: "Drift",
            width: 120,
            filterable: false,
            sortable: false,
            valueGetter: rp => {
                if (!rp.row.ts.commandResults.length) {
                    return ""
                }
                const rs = rp.row.ts.commandResults[0]
                return rs.commandInfo.command
            },
            renderCell: rp => {
                const dr = rp.row.ts.lastDriftDetectionResult
                return <Box display={"flex"}
                            width={"100%"}
                            onDoubleClick={() => doSelectTarget(rp.row)}>
                    <DriftDetectionResultStatusLine dr={dr}/>
                    {dr?.objects?.length && <ManualApproveButton ts={rp.row.ts} renderedObjectsHash={dr.renderedObjectsHash!}/>}
                </Box>
            }
        },
        {
            field: "lastCommandResult",
            headerName: "Last Command",
            width: 120,
            filterable: false,
            sortable: false,
            valueGetter: rp => {
                if (!rp.row.ts.commandResults.length) {
                    return ""
                }
                const rs = rp.row.ts.commandResults[0]
                return rs.commandInfo.command
            },
            renderCell: rp => {
                if (!rp.row.ts.commandResults.length) {
                    return <></>
                }
                const rs = rp.row.ts.commandResults[0]
                return <Box display={"flex"}
                            width={"100%"}
                            onDoubleClick={() => doSelectCommandResult(rp.row, 0)}>
                    <CommandResultStatusLine rs={rs}/>
                    <Tooltip title={<Typography>Show full result tree</Typography>}>
                    <IconButton
                        onClick={e => {
                            e.stopPropagation();
                            doSelectCommandResult(rp.row, 0, true)
                        }}
                        sx={{
                            padding: 0,
                            width: 26,
                            height: 26
                        }}
                    >
                            <TreeViewIcon />
                    </IconButton>
                    </Tooltip>
                </Box>
            }
        },
        {
            field: "lastCommandResultTime",
            headerName: "When",
            width: 100,
            type: "dateTime",
            valueGetter: rp => {
                if (!rp.row.ts.commandResults.length) {
                    return undefined
                }
                const rs = rp.row.ts.commandResults[0]
                return new Date(rs.commandInfo.startTime)
            },
            renderCell: rp => {
                if (!rp.row.ts.commandResults.length) {
                    return <></>
                }
                const rs = rp.row.ts.commandResults[0]
                return <Box onDoubleClick={() => doSelectCommandResult(rp.row, 0)}>
                    <Since startTime={new Date(rs.commandInfo.startTime)}/>
                </Box>
            }
        },
        {
            field: "actions",
            headerName: "Actions",
            width: 96,
            type: "actions",
            renderCell: rp => {
                return <TargetActionMenu ts={rp.row.ts}/>
            },
        },
    ];

    const rows = useMemo(() => {
        const rows: Row[] = []
        projects.forEach(ps => {
            ps.targets.forEach(ts => {
                const key = buildTargetKey(ps.project, ts.target, ts.kdInfo)
                rows.push({
                    id: key,
                    ps: ps,
                    ts: ts,
                })
            })
        })
        return rows
    }, [projects])

    let dialogCard: React.ReactElement | undefined
    if (props.selectedProject && props.selectedTarget) {
        if (!props.selectedResult) {
            dialogCard = <TargetCard
                ps={props.selectedProject}
                ts={props.selectedTarget}
                expanded={true}
                onClose={() => props.onCloseExpanded()}
            />
        } else {
            dialogCard = <CommandResultCard
                current={true}
                ps={props.selectedProject}
                ts={props.selectedTarget}
                rs={props.selectedResult}
                onSwitchFullCommandResult={() => doSelect(props.selectedProject!, props.selectedTarget!, true, props.selectedResult, !props.selectedResultFull)}
                showSummary={!props.selectedResultFull}
                expanded={true}
                loadData={true}
                onClose={() => props.onCloseExpanded()}
            />
        }
    }

    const dialogPaperProps = {
        sx: {
            height: "100%"
        }
    }

    return <Box>
        <Dialog
            open={!!dialogCard}
            maxWidth={"xl"}
            fullWidth={true}
            PaperProps={dialogPaperProps}
            onClose={() => props.onCloseExpanded()}
        >
            {dialogCard}
        </Dialog>
        <DataGrid
            rows={rows}
            columns={columns}
            initialState={{
                pagination: {
                    paginationModel: { page: 0, pageSize: 50 },
                },
            }}
            pageSizeOptions={[5, 50]}
        />
    </Box>
}

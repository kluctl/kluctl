import { buildTargetKey, ProjectSummary, TargetSummary } from "../../project-summaries";
import { CommandResultSummary } from "../../models";
import { useAppContext } from "../App";
import React, { useCallback, useMemo } from "react";
import { Box, Dialog, Typography } from "@mui/material";
import { DataGrid, GridColDef, gridStringOrNumberComparator } from '@mui/x-data-grid';
import { getLastPathElement } from "../../utils/misc";
import Tooltip from "@mui/material/Tooltip";
import { Approval, Toys } from "@mui/icons-material";
import { ReconcilingIcon } from "../target-view/ReconcilingIcon";
import { StatusIcon } from "../target-view/StatusIcon";
import { TargetActionMenu } from "../target-view/TargetActionMenu";
import { CommandResultStatusLine } from "../command-result/CommandResultStatusLine";
import { Since } from "../Since";
import { ScrollingTextLine } from "../ScrollingTextLine";
import { ClusterIcon } from "../target-view/ClusterIcon";
import { CommandTypeIcon } from "../target-view/CommandTypeIcon";
import Divider from "@mui/material/Divider";
import { TargetCard } from "../target-cards-view/TargetCard";
import { CommandResultCard } from "../command-result/CommandResultCard";


export interface TargetsListViewProps {
    selectedProject?: ProjectSummary
    selectedTarget?: TargetSummary
    selectedResult?: CommandResultSummary
    onSelect: (ps: ProjectSummary, ts: TargetSummary, showResults: boolean, rs?: CommandResultSummary) => void
    onCloseExpanded: () => void
}

export const TargetsListView = (props: TargetsListViewProps) => {
    const appContext = useAppContext();
    const projects = appContext.projects;

    const selectedTargetKey = useMemo(() => {
        if (!props.selectedProject || !props.selectedTarget) {
            return undefined
        }
        return buildTargetKey(props.selectedProject.project, props.selectedTarget.target, props.selectedTarget.kdInfo)
    }, [props.selectedProject, props.selectedTarget])

    interface Row {
        id: string,
        ps: ProjectSummary
        ts: TargetSummary
    }

    const doSelect = useCallback((ps: ProjectSummary, ts: TargetSummary, showResults: boolean, rs?: CommandResultSummary) => {
        console.log("select", ps, ts, showResults, rs)
        props.onSelect(ps, ts, showResults, rs)
    }, [])

    const doSelectTarget = (row: Row) => {
        doSelect(row.ps, row.ts, false)
    }
    const doSelectCommandResult = (row: Row, index: number) => {
        if (index >= (row.ts.commandResults.length || 0)) {
            return
        }
        const rs = row.ts.commandResults[index]
        doSelect(row.ps, row.ts, true, rs)
    }

    const columns: GridColDef<Row>[] = [
        {
            field: "project",
            headerName: "Project",
            width: 200,
            valueGetter: rp => {
                return rp.row.ps.project.gitRepoKey
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
            renderCell: rp => {
                return <Box onDoubleClick={() => doSelectTarget(rp.row)}>
                    <StatusIcon ps={rp.row.ps} ts={rp.row.ts}/>
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
            field: "lastCommandResult",
            headerName: "Last Command",
            width: 120,
            filterable: false,
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
                    <CommandTypeIcon ts={rp.row.ts} rs={rs} size={"24px"}/>
                    <Divider orientation={"vertical"} sx={{height: "inherit", marginX: "5px"}}/>
                    <CommandResultStatusLine rs={rs}/>
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
                onSwitchFullCommandResult={() => {}}
                showSummary={true}
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
            maxWidth={"lg"}
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

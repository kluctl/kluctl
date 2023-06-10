import { CommandResultSummary } from "../../models";
import { api, usePromise } from "../../api";
import { NodeBuilder } from "../result-view/nodes/NodeBuilder";
import React, { Suspense, useEffect, useMemo, useRef, useState } from "react";
import { NodeData } from "../result-view/nodes/NodeData";
import { SidePanel } from "../result-view/SidePanel";
import { Box, Drawer, ThemeProvider, useTheme } from "@mui/material";
import { Loading } from "../Loading";
import { dark } from "../theme";
import { Card, cardGap, cardHeight, cardWidth } from "./Card";
import { CommandResultItem } from "./CommandResultItem";
import { ProjectSummary, TargetSummary } from "../../project-summaries";

const sidePanelWidth = 720;

async function doGetRootNode(rs: CommandResultSummary) {
    const shortNames = api.getShortNames()
    const r = api.getResult(rs.id)
    const builder = new NodeBuilder({
        shortNames: await shortNames,
        summary: rs,
        commandResult: await r,
    })
    const [node] = builder.buildRoot()
    return node
}

export const CommandResultDetailsDrawer = React.memo((props: {
    rs?: CommandResultSummary,
    ts?: TargetSummary,
    ps?: ProjectSummary,
    onClose: () => void,
    selectedCardRect?: DOMRect
}) => {
    const { ps, ts, selectedCardRect } = props;
    const theme = useTheme();
    const [promise, setPromise] = useState<Promise<NodeData>>(new Promise(() => undefined));
    const [selectedCommandResult, setSelectedCommandResult] = useState<CommandResultSummary | undefined>();
    const [prevTargetSummary, setPrevTargetSummary] = useState<TargetSummary | undefined>(ts);
    const [cardsCoords, setCardsCoords] = useState<{ left: number, top: number }[]>([]);
    const [transitionRunning, setTransitionRunning] = useState(false);

    if (prevTargetSummary !== ts) {
        setPrevTargetSummary(ts);
        setSelectedCommandResult(ts?.commandResults?.[0]);
    }

    useEffect(() => {
        if (selectedCommandResult === undefined) {
            return
        }
        setPromise(doGetRootNode(selectedCommandResult));
    }, [selectedCommandResult])

    const Content = (props: { onClose: () => void }) => {
        const node = usePromise(promise)
        return <SidePanel provider={node} onClose={props.onClose} />
    }

    const cardsContainerElem = useRef<HTMLElement>();

    useEffect(() => {
        const rect = cardsContainerElem.current?.getBoundingClientRect();
        if (!rect || !selectedCardRect || !ts?.commandResults) {
            setCardsCoords([]);
            return;
        }

        const initialCoords = ts.commandResults.map(() => ({
            left: selectedCardRect.left - rect.left,
            top: selectedCardRect.top - rect.top
        }));

        setCardsCoords(initialCoords);
        setTransitionRunning(true);
    }, [selectedCardRect, ts?.commandResults]);

    useEffect(() => {
        if (cardsCoords.length > 0) {
            const targetCoords = cardsCoords.map((_, i) => ({
                left: 0,
                top: i * (cardHeight + cardGap)
            }));

            if (cardsCoords.length === targetCoords.length
                && cardsCoords.every(({ left, top }, i) =>
                    targetCoords[i].left === left && targetCoords[i].top === top
                )
            ) {
                return;
            }

            setTimeout(() => {
                setCardsCoords(targetCoords);
                setTimeout(() => {
                    setTransitionRunning(false);
                }, theme.transitions.duration.enteringScreen)
            }, 10);
        }
    }, [cardsCoords, theme.transitions.duration.enteringScreen])

    const zIndex = theme.zIndex.modal + 1;

    const cards = useMemo(() => {
        if (!(ps && ts && ts.commandResults && ts.commandResults.length > 0 && cardsCoords.length > 0)) {
            return null;
        }

        return ts.commandResults.map((rs, i) => {
            return <Card
                key={rs.id}
                sx={{
                    position: 'absolute',
                    translate: `${cardsCoords[i].left}px ${cardsCoords[i].top}px`,
                    zIndex: zIndex,
                    transition: theme.transitions.create(['translate'], {
                        easing: theme.transitions.easing.sharp,
                        duration: theme.transitions.duration.enteringScreen,
                    })
                }}
            >
                <CommandResultItem
                    ps={ps}
                    ts={ts}
                    rs={rs}
                    onSelectCommandResult={setSelectedCommandResult}
                    selected={rs === selectedCommandResult}
                />
            </Card>
        })
    }, [ps, ts, cardsCoords, zIndex, theme.transitions, selectedCommandResult])

    return <>
        {ps && ts && ts.commandResults && ts.commandResults.length > 0 &&
            <Box
                sx={{
                    position: 'fixed',
                    top: 0,
                    bottom: 0,
                    right: sidePanelWidth,
                    width: `calc(100% - ${sidePanelWidth}px)`,
                    overflowX: 'visible',
                    overflowY: transitionRunning ? 'visible' : 'auto',
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
                    padding: '25px 0',
                    zIndex: zIndex
                }}
                onClick={props.onClose}
            >
                <Box
                    onClick={(e) => e.stopPropagation()}
                    position='relative'
                    width={cardWidth}
                    height={cardHeight * ts.commandResults.length + cardGap * (ts.commandResults.length - 1)}
                    ref={cardsContainerElem}
                >
                    {cards}
                </Box>
            </Box>
        }
        <ThemeProvider theme={dark}>
            <Drawer
                sx={{ zIndex: 1300 }}
                anchor={"right"}
                open={props.rs !== undefined}
                onClose={props.onClose}
            >
                <Box width={sidePanelWidth} height={"100%"}>
                    <Suspense fallback={<Loading />}>
                        <Content onClose={props.onClose} />
                    </Suspense>
                </Box>
            </Drawer>
        </ThemeProvider>
    </>
});

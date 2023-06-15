import React, { useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { Box, Divider, IconButton, SxProps, Tab, Tooltip, useTheme } from "@mui/material";
import { CommandResultSummary } from "../../models";
import { TargetSummary } from "../../project-summaries";
import { CardPaper, cardHeight, cardWidth } from "./Card";
import { CommandResultItemHeader } from "./CommandResultItem";
import { Loading, useLoadingHelper } from "../Loading";
import { NodeBuilder } from "../result-view/nodes/NodeBuilder";
import { SidePanelProvider, useSidePanelTabs } from "../result-view/SidePanel";
import { TabContext, TabList, TabPanel } from "@mui/lab";
import { Api } from "../../api";
import { ApiContext } from "../App";
import { useNavigate } from "react-router";
import { CloseLightIcon, TreeViewIcon, TriangleLeftLightIcon, TriangleRightLightIcon } from "../../icons/Icons";

async function doGetRootNode(api: Api, rs: CommandResultSummary) {
    const shortNames = await api.getShortNames()
    const r = await api.getResult(rs.id)
    const builder = new NodeBuilder({
        shortNames: shortNames,
        summary: rs,
        commandResult: r,
    })
    const [node] = builder.buildRoot()
    return node
}

export interface HistoryCardsProps {
    rs: CommandResultSummary,
    ts: TargetSummary,
    initialCardRect: DOMRect,
    onClose: () => void
}

interface Rect {
    left: number,
    top: number,
    width: number | string,
    height: number | string
}

type TransitionStatus = 'not-started' | 'running' | 'finished'

const CardContent = React.memo((props: { provider: SidePanelProvider }) => {
    const { tabs, selectedTab, handleTabChange } = useSidePanelTabs(props.provider)

    if (!props.provider
        || !selectedTab
        || !tabs.find(x => x.label === selectedTab)
    ) {
        return null;
    }

    return <TabContext value={selectedTab}>
        <Box height='36px' flex='0 0 auto' p='0 30px' mt='12px'>
            <TabList onChange={handleTabChange}>
                {tabs.map((tab, i) => {
                    return <Tab label={tab.label} value={tab.label} key={tab.label} />
                })}
            </TabList>
        </Box>
        <Divider sx={{ margin: 0 }} />
        <Box overflow='auto' p='30px'>
            {tabs.map(tab => {
                return <TabPanel key={tab.label} value={tab.label} sx={{ padding: 0 }}>
                    {tab.content}
                </TabPanel>
            })}
        </Box>
    </TabContext>
});

const arrowButtonWidth = 80;

const ArrowButton = React.memo((props: {
    direction: 'left' | 'right',
    onClick: () => void,
    hidden: boolean
}) => {
    const Icon = {
        left: TriangleLeftLightIcon,
        right: TriangleRightLightIcon
    }[props.direction];

    return <Box
        flex='0 0 auto'
        height='100%'
        width={`${arrowButtonWidth}px`}
        display='flex'
        justifyContent='center'
        alignItems='center'
        position='relative'
        {...{ [props.direction]: 0 }}
    >
        {!props.hidden &&
            <IconButton onClick={props.onClick}>
                <Icon />
            </IconButton>
        }
    </Box>
});

const HistoryCard = React.memo((props: {
    rs: CommandResultSummary,
    sx?: SxProps
    transitionFinished?: boolean,
    onClose?: () => void;
}) => {
    const navigate = useNavigate();
    const api = useContext(ApiContext);
    const [loading, loadingError, node] = useLoadingHelper(() => {
        return doGetRootNode(api, props.rs)
    }, [api, props.rs]);

    if (loadingError) {
        return <>Error</>
    }

    return <CardPaper
        sx={{
            position: 'relative',
            ...props.sx
        }}
    >
        <Box
            position='absolute'
            right='10px'
            top='10px'
        >
            {props.transitionFinished && (
                <IconButton onClick={props.onClose}>
                    <CloseLightIcon />
                </IconButton>
            )}
        </Box>
        <Box
            display='flex'
            flexDirection='column'
            height='100%'
            justifyContent='space-between'
        >
            <Box p='0 16px' flex='0 0 auto'>
                <CommandResultItemHeader rs={props.rs} />
            </Box>
            <Box width='100%' flex='1 1 auto' overflow='hidden' display='flex' flexDirection='column'>
                {props.transitionFinished && (
                    loading
                        ? <Loading />
                        : <CardContent provider={node!} />
                )}
            </Box>
            <Box
                flex='0 0 auto'
                height='39px'
                display='flex'
                alignItems='center'
                justifyContent='end'
                p='0 30px'
            >
                <IconButton
                    onClick={e => {
                        e.stopPropagation();
                        navigate(`/results/${props.rs.id}`);
                    }}
                    sx={{
                        padding: 0,
                        width: 32,
                        height: 32
                    }}
                >
                    <Tooltip title='Open Result Tree'>
                        <Box display='flex'><TreeViewIcon /></Box>
                    </Tooltip>
                </IconButton>
            </Box>
        </Box>
    </CardPaper>
});

export const HistoryCards = React.memo((props: HistoryCardsProps) => {
    const theme = useTheme();
    const containerElem = useRef<HTMLElement>();
    const [cardRect, setCardRect] = useState<Rect | undefined>();
    const [transitionStatus, setTransitionStatus] = useState<TransitionStatus>('not-started');
    const [currentRS, setCurrentRS] = useState(props.rs);

    useEffect(() => {
        const rect = containerElem.current?.getBoundingClientRect();
        if (!rect) {
            setCardRect(undefined);
            return;
        }

        const initialRect = {
            left: props.initialCardRect.left - rect.left,
            top: props.initialCardRect.top - rect.top,
            width: cardWidth,
            height: cardHeight
        };

        setCardRect(initialRect);
    }, [props.initialCardRect]);

    useEffect(() => {
        if (!cardRect) {
            return;
        }

        const targetRect = {
            left: 0,
            top: 0,
            width: '100%',
            height: '100%'
        };

        if (cardRect.left === targetRect.left
            && cardRect.top === targetRect.top
            && cardRect.width === targetRect.width
            && cardRect.height === targetRect.height
        ) {
            return;
        }

        setTimeout(() => {
            setCardRect(targetRect);
            setTransitionStatus('running');
            setTimeout(() => {
                setTransitionStatus('finished');
            }, theme.transitions.duration.enteringScreen);
        }, 10);
    }, [cardRect, theme.transitions.duration.enteringScreen]);

    const currentRSIndex = useMemo(
        () => props.ts.commandResults.indexOf(currentRS),
        [currentRS, props.ts.commandResults]
    )

    const onLeftArrowClick = useCallback(() => {
        if (currentRSIndex > 0) {
            setCurrentRS(props.ts.commandResults[currentRSIndex - 1]);
        }
    }, [currentRSIndex, props.ts.commandResults]);

    const onRightArrowClick = useCallback(() => {
        if (currentRSIndex < props.ts.commandResults.length - 1) {
            setCurrentRS(props.ts.commandResults[currentRSIndex + 1]);
        }
    }, [currentRSIndex, props.ts.commandResults]);

    const paddingX = 40;
    const gap = 2 * (paddingX + arrowButtonWidth);

    return <Box
        width='100%'
        height='100%'
        p={`25px ${paddingX}px`}
        display='flex'
        position='relative'
        overflow='hidden'
    >
        <ArrowButton
            direction='left'
            onClick={onLeftArrowClick}
            hidden={currentRSIndex === 0 || transitionStatus !== 'finished'}
        />
        <Box
            flex='0 0 auto'
            width={`calc(100% - ${arrowButtonWidth}px * 2)`}
            display='flex'
            ref={containerElem}
        >
            {transitionStatus !== 'finished' && cardRect && <HistoryCard
                sx={{
                    width: cardRect.width,
                    height: cardRect.height,
                    flex: '0 0 auto',
                    translate: `${cardRect.left}px ${cardRect.top}px`,
                    transition: theme.transitions.create(['translate', 'width', 'height'], {
                        easing: theme.transitions.easing.sharp,
                        duration: theme.transitions.duration.enteringScreen,
                    }),
                    padding: '20px 0'
                }}
                rs={currentRS}
                onClose={props.onClose}
            />}
            {transitionStatus === 'finished' &&
                <Box
                    flex='1 1 auto'
                    width='100%'
                    height='100%'
                    display='flex'
                    gap={`${gap}px`}
                    sx={{
                        translate: `calc((-100% - ${gap}px) * ${currentRSIndex})`,
                        transition: theme.transitions.create(['translate'], {
                            easing: theme.transitions.easing.sharp,
                            duration: theme.transitions.duration.enteringScreen,
                        })
                    }}
                >
                    {props.ts.commandResults.map((rs) =>
                        <HistoryCard
                            sx={{
                                width: '100%',
                                height: '100%',
                                flex: '0 0 auto',
                                padding: '20px 0'
                            }}
                            rs={rs}
                            key={rs.id}
                            transitionFinished={transitionStatus === 'finished'}
                            onClose={props.onClose}
                        />
                    )}
                </Box>
            }
        </Box>
        <ArrowButton
            direction='right'
            onClick={onRightArrowClick}
            hidden={currentRSIndex === props.ts.commandResults.length - 1 || transitionStatus !== 'finished'}
        />
    </Box>;
});

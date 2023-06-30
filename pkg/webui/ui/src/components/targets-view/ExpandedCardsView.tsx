import React, { useCallback, useEffect, useRef, useState } from "react";
import { Box, ClickAwayListener, IconButton, SxProps, Theme, useTheme } from "@mui/material";
import { cardHeight, cardWidth } from "./Card";
import { TriangleLeftLightIcon, TriangleRightLightIcon } from "../../icons/Icons";

interface Rect {
    left: number,
    top: number,
    width: number | string,
    height: number | string
}

const arrowButtonWidth = 80;

const ArrowButton = React.memo((props: {
    direction: 'left' | 'right',
    onButtonClick: () => void,
    onContainerClick: () => void,
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
        onClick={props.onContainerClick}
    >
        {!props.hidden &&
            <IconButton
                onClick={(e) => {
                    e.stopPropagation();
                    props.onButtonClick()
                }}
            >
                <Icon />
            </IconButton>
        }
    </Box>
});

type TransitionState =
    {
        type: 'initial'
    } | {
        type: 'started',
        cardRect: Rect
    } | {
        type: 'running',
        cardRect: Rect
    } | {
        type: 'finished'
    }

export interface ExpandedCardsViewProps<CardData> {
    initialCardRect?: DOMRect,
    cardsData: CardData[],
    renderCard: (
        cardData: CardData,
        sx: SxProps<Theme>,
        expanded: boolean,
        current: boolean
    ) => React.ReactNode,
    onClose: () => void
}

export const ExpandedCardsView = function <CardData>(props: ExpandedCardsViewProps<CardData>) {
    const theme = useTheme();
    const containerElem = useRef<HTMLElement>();
    const [transitionState, setTransitionState] = useState<TransitionState>({ type: 'initial' });
    const [currentIndex, setCurrentIndex] = useState(0);

    useEffect(() => {
        const rect = containerElem.current?.getBoundingClientRect();
        if (!rect) {
            setTransitionState({ type: 'initial' });
            return;
        }

        if (!props.initialCardRect) {
            setTransitionState({ type: 'finished' });
            return;
        }

        const initialRect = {
            left: props.initialCardRect.left - rect.left,
            top: props.initialCardRect.top - rect.top,
            width: cardWidth,
            height: cardHeight
        };

        setTransitionState({
            type: 'started',
            cardRect: initialRect
        });
    }, [props.initialCardRect]);

    useEffect(() => {
        if (transitionState.type !== 'started') {
            return;
        }

        const targetRect = {
            left: 0,
            top: 0,
            width: '100%',
            height: '100%'
        };

        setTimeout(() => {
            setTransitionState({
                type: 'running',
                cardRect: targetRect
            });
            setTimeout(() => {
                setTransitionState({ type: 'finished' });
            }, theme.transitions.duration.enteringScreen);
        }, 10);
    }, [transitionState, theme.transitions.duration.enteringScreen]);

    const onLeftArrowClick = useCallback(() => {
        if (currentIndex > 0) {
            setCurrentIndex(i => i - 1);
        }
    }, [currentIndex]);

    const onRightArrowClick = useCallback(() => {
        if (currentIndex < props.cardsData.length - 1) {
            setCurrentIndex(i => i + 1);
        }
    }, [currentIndex, props.cardsData.length]);

    const paddingX = 40;
    const gap = 2 * (paddingX + arrowButtonWidth);

    return <Box
        width='100%'
        height='100%'
        p={`25px ${paddingX}px`}
    >
        <ClickAwayListener onClickAway={props.onClose}>
            <Box
                width='100%'
                height='100%'
                display='flex'
                position='relative'
                overflow='hidden'
            >
                <ArrowButton
                    direction='left'
                    onButtonClick={onLeftArrowClick}
                    onContainerClick={props.onClose}
                    hidden={currentIndex === 0 || transitionState.type !== 'finished'}
                />
                <Box
                    flex='0 0 auto'
                    width={`calc(100% - ${arrowButtonWidth}px * 2)`}
                    display='flex'
                    ref={containerElem}
                >
                    {(transitionState.type === 'started' || transitionState.type === 'running') &&
                        props.renderCard(
                            props.cardsData[0],
                            {
                                width: transitionState.cardRect.width,
                                height: transitionState.cardRect.height,
                                translate: `${transitionState.cardRect.left}px ${transitionState.cardRect.top}px`,
                                transition: theme.transitions.create(['translate', 'width', 'height'], {
                                    easing: theme.transitions.easing.sharp,
                                    duration: theme.transitions.duration.enteringScreen,
                                }),
                            },
                            false,
                            false
                        )
                    }
                    {transitionState.type === 'finished' &&
                        <Box
                            flex='1 1 auto'
                            width='100%'
                            height='100%'
                            display='flex'
                            gap={`${gap}px`}
                            sx={{
                                translate: `calc((-100% - ${gap}px) * ${currentIndex})`,
                                transition: theme.transitions.create(['translate'], {
                                    easing: theme.transitions.easing.sharp,
                                    duration: theme.transitions.duration.enteringScreen,
                                })
                            }}
                        >
                            {props.cardsData.map((cd, i) =>
                                props.renderCard(
                                    cd,
                                    {
                                        width: '100%',
                                        height: '100%',
                                    },
                                    true,
                                    i === currentIndex
                                )
                            )}
                        </Box>
                    }
                </Box>
                <ArrowButton
                    direction='right'
                    onButtonClick={onRightArrowClick}
                    onContainerClick={props.onClose}
                    hidden={
                        currentIndex === props.cardsData.length - 1
                        || transitionState.type !== 'finished'
                    }
                />
            </Box>
        </ClickAwayListener>
    </Box>;
};

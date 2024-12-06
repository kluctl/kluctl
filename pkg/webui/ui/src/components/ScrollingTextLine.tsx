import React, { useCallback, useRef, useState } from "react";
import { Box, keyframes, Typography, TypographyProps } from "@mui/material"

export type ScrollingTextLineProps = Omit<TypographyProps, 'children'> & {
    children: React.ReactNode;
    scrollPadding?: number;
    scrollSpeed?: number; 
}

export const ScrollingTextLine = React.forwardRef((
    props: ScrollingTextLineProps,
    forwardedRef: React.ForwardedRef<HTMLElement>
) => {
    const {
        children,
        scrollPadding = 10,
        scrollSpeed = 200,
        ...rest 
    } = props;
    const containerElem = useRef<HTMLElement | null>();
    const scrollingElem = useRef<HTMLElement | null>();

    const [containerElemWidth, setContainerElemWidth] = useState<number | undefined>();
    const [scrollingElemWidth, setScrollingElemWidth] = useState<number | undefined>();

    const onMouseEnter = useCallback(() => {
        setContainerElemWidth(containerElem.current?.getBoundingClientRect().width);
        setScrollingElemWidth(scrollingElem.current?.getBoundingClientRect().width);
    }, []);

    const maxScrollDistance = (containerElemWidth
        && scrollingElemWidth
        && containerElemWidth < scrollingElemWidth)
        ? Math.round(containerElemWidth - scrollingElemWidth)
        : undefined;

    const animation = maxScrollDistance !== undefined
        ? keyframes`
                from {
                    translate: ${scrollPadding}px;
                }
                to {
                    translate: ${maxScrollDistance - scrollPadding}px;
                }
            `
        : undefined;

    const duration = (scrollingElemWidth || 0) / scrollSpeed;

    return <Typography
        textAlign='left'
        textOverflow='ellipsis'
        overflow='hidden'
        whiteSpace='nowrap'
        {...rest}
        ref={(r) => {
            if (typeof forwardedRef === 'function') {
                forwardedRef(r);
            } else if (forwardedRef !== null) {
                forwardedRef.current = r;
            }
            containerElem.current = r;
        }}
    >
        <Box
            component='span'
            sx={{
                display: 'inline',
                '&:hover': {
                    display: 'inline-block',
                    ...(animation && {
                        animation: `${animation} ${duration}s infinite alternate linear`
                    })
                },
            }}
            ref={scrollingElem}
            onMouseEnter={onMouseEnter}
        >
            {children}
        </Box>
    </Typography>;
});

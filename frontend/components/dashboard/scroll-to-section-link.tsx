"use client";

import type { MouseEvent, ReactNode } from "react";

type ScrollToSectionLinkProps = {
    targetId: string;
    className?: string;
    children: ReactNode;
};

// Scrolls to a section on the same page and moves focus to it, so keyboard
// and screen reader users land on the section instead of staying on the link.
export function ScrollToSectionLink({
    targetId,
    className,
    children,
}: ScrollToSectionLinkProps) {
    function handleClick(event: MouseEvent<HTMLAnchorElement>) {
        const target = document.getElementById(targetId);

        // Let the browser handle new-tab clicks, or fall back to the native
        // hash navigation if the section is missing.
        if (
            !target ||
            event.metaKey ||
            event.ctrlKey ||
            event.shiftKey ||
            event.altKey
        ) {
            return;
        }

        event.preventDefault();

        const prefersReducedMotion = window.matchMedia(
            "(prefers-reduced-motion: reduce)",
        ).matches;

        target.scrollIntoView({
            behavior: prefersReducedMotion ? "auto" : "smooth",
            block: "start",
        });

        // preventScroll keeps focus() from cutting the smooth scroll short.
        target.focus({ preventScroll: true });
    }

    return (
        <a href={`#${targetId}`} onClick={handleClick} className={className}>
            {children}
        </a>
    );
}

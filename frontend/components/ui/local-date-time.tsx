"use client";

import { useSyncExternalStore } from "react";

// The formatted value never needs to re-render on its own, so there is
// nothing to subscribe to.
function subscribe() {
    return () => {};
}

// Formats the instant in the browser's timezone.
export function formatLocalDateTime(value: string): string {
    return new Date(value).toLocaleString("en-GB");
}

// The server does not know the user's timezone, so it renders nothing and
// the browser fills in the date from its local clock after hydration.
function getServerSnapshot() {
    return null;
}

type LocalDateTimeProps = {
    value: string;
};

export function LocalDateTime({ value }: LocalDateTimeProps) {
    const formatted = useSyncExternalStore(
        subscribe,
        () => formatLocalDateTime(value),
        getServerSnapshot,
    );

    return (
        <time dateTime={value}>
            {/* A non-breaking space keeps the line height while empty. */}
            {formatted ?? " "}
        </time>
    );
}

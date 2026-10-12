"use client";

import { useSyncExternalStore } from "react";

import { getGreeting } from "@/lib/greeting";

// The greeting never needs to re-render on its own, so there is nothing to
// subscribe to.
function subscribe() {
    return () => {};
}

function getClientGreeting() {
    return getGreeting(new Date().getHours());
}

// The server does not know the user's timezone, so it renders nothing and
// the browser fills in the greeting from its local clock after hydration.
function getServerGreeting() {
    return null;
}

export function Greeting() {
    const greeting = useSyncExternalStore(
        subscribe,
        getClientGreeting,
        getServerGreeting,
    );

    return (
        <h2 className="text-2xl font-semibold tracking-tight text-slate-900">
            {/* A non-breaking space keeps the heading height while empty. */}
            {greeting ?? " "}
        </h2>
    );
}

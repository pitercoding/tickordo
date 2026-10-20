const DEV_API_URL = "http://localhost:8080";

// Requests that take longer than this are aborted with an ApiTimeoutError.
export const DEFAULT_TIMEOUT_MS = 15_000;

// NEXT_PUBLIC_* variables are inlined at build time, so the variable must be
// read with this literal expression and a missing value has to fail the
// production build instead of shipping a bundle that calls localhost.
function resolveApiUrl(): string {
    const configuredUrl = process.env.NEXT_PUBLIC_API_URL?.trim();

    if (!configuredUrl) {
        if (process.env.NODE_ENV === "production") {
            throw new Error(
                "NEXT_PUBLIC_API_URL must be set for production builds (e.g. https://api.example.com).",
            );
        }

        return DEV_API_URL;
    }

    // A trailing slash would turn "/tickets" into "//tickets".
    return configuredUrl.replace(/\/+$/, "");
}

const API_URL = resolveApiUrl();

export class ApiError extends Error {
    constructor(
        message: string,
        public readonly status: number,
    ) {
        super(message);
        this.name = "ApiError";
    }
}

export class ApiTimeoutError extends Error {
    constructor(public readonly timeoutMs: number) {
        super(`API request timed out after ${timeoutMs} ms`);
        this.name = "ApiTimeoutError";
    }
}

export type ApiRequestOptions = RequestInit & {
    // Overrides DEFAULT_TIMEOUT_MS for slow endpoints such as AI triage.
    timeoutMs?: number;
};

export async function apiClient<T>(
    endpoint: string,
    options?: ApiRequestOptions,
): Promise<T> {
    const { timeoutMs = DEFAULT_TIMEOUT_MS, ...init } = options ?? {};
    const timeoutSignal = AbortSignal.timeout(timeoutMs);

    try {
        const response = await fetch(`${API_URL}${endpoint}`, {
            ...init,
            headers: {
                "Content-Type": "application/json",
                ...init.headers,
            },
            // Keep a caller's signal working alongside the timeout.
            signal: init.signal
                ? AbortSignal.any([init.signal, timeoutSignal])
                : timeoutSignal,
        });

        if (!response.ok) {
            // The API returns errors as { "error": "message" }.
            const body = await response.json().catch(() => null);
            const message =
                typeof body?.error === "string"
                    ? body.error
                    : `API request failed with status ${response.status}`;

            throw new ApiError(message, response.status);
        }

        // Some successful responses, such as HTTP 204, have no body.
        if (response.status === 204) {
            return undefined as T;
        }

        // Awaited here so a timeout while reading the body is caught below.
        return await response.json();
    } catch (error) {
        // fetch and response.json() reject with the signal's reason, a
        // TimeoutError DOMException, when the timeout fires. Other network
        // errors and ApiError are rethrown unchanged.
        if (error instanceof Error && error.name === "TimeoutError") {
            throw new ApiTimeoutError(timeoutMs);
        }

        throw error;
    }
}

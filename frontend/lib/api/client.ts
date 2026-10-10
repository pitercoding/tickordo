const API_URL = "http://localhost:8080";

export class ApiError extends Error {
    constructor(
        message: string,
        public readonly status: number,
    ) {
        super(message);
        this.name = "ApiError";
    }
}

export async function apiClient<T>(
    endpoint: string,
    options?: RequestInit,
): Promise<T> {
    const response = await fetch(`${API_URL}${endpoint}`, {
        ...options,
        headers: {
            "Content-Type": "application/json",
            ...options?.headers,
        },
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

    return response.json();
}

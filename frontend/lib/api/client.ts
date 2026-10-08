const API_URL = "http://localhost:8080";

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
        throw new Error(`API request failed with status ${response.status}`);
    }

    return response.json();
}

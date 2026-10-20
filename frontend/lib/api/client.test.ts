import { beforeEach, describe, expect, it, vi } from "vitest";

const fetchMock = vi.fn<typeof fetch>();

// API_URL is read when the module is evaluated, so each test sets the
// environment first and then imports a fresh copy of the client.
async function loadClient(apiUrl?: string) {
    vi.stubEnv("NEXT_PUBLIC_API_URL", apiUrl);
    vi.resetModules();
    return import("./client");
}

function jsonResponse(body: unknown, status = 200): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
    });
}

beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
});

describe("apiClient", () => {
    describe("request URL", () => {
        it("prefixes the endpoint with NEXT_PUBLIC_API_URL", async () => {
            const { apiClient } = await loadClient("https://api.example.com");
            fetchMock.mockResolvedValue(jsonResponse([]));

            await apiClient("/tickets");

            expect(fetchMock).toHaveBeenCalledOnce();
            expect(fetchMock.mock.calls[0][0]).toBe(
                "https://api.example.com/tickets",
            );
        });

        it.each([
            ["unset", undefined],
            ["empty", ""],
        ])(
            "falls back to the local API when the variable is %s",
            async (_, apiUrl) => {
                const { apiClient } = await loadClient(apiUrl);
                fetchMock.mockResolvedValue(jsonResponse({}));

                await apiClient("/dashboard/stats");

                expect(fetchMock.mock.calls[0][0]).toBe(
                    "http://localhost:8080/dashboard/stats",
                );
            },
        );

        it.each([
            ["unset", undefined],
            ["empty", ""],
            ["blank", "   "],
        ])(
            "fails in production when the variable is %s",
            async (_, apiUrl) => {
                vi.stubEnv("NODE_ENV", "production");

                await expect(loadClient(apiUrl)).rejects.toThrow(
                    "NEXT_PUBLIC_API_URL must be set",
                );
            },
        );

        it("uses the configured URL in production", async () => {
            vi.stubEnv("NODE_ENV", "production");
            const { apiClient } = await loadClient("https://api.example.com");
            fetchMock.mockResolvedValue(jsonResponse([]));

            await apiClient("/tickets");

            expect(fetchMock.mock.calls[0][0]).toBe(
                "https://api.example.com/tickets",
            );
        });

        it.each([
            ["one trailing slash", "https://api.example.com/"],
            ["several trailing slashes", "https://api.example.com//"],
        ])("removes %s from the base URL", async (_, apiUrl) => {
            const { apiClient } = await loadClient(apiUrl);
            fetchMock.mockResolvedValue(jsonResponse([]));

            await apiClient("/tickets");

            expect(fetchMock.mock.calls[0][0]).toBe(
                "https://api.example.com/tickets",
            );
        });
    });

    describe("request options", () => {
        it("sends a GET with the JSON content type by default", async () => {
            const { apiClient } = await loadClient("https://api.example.com");
            fetchMock.mockResolvedValue(jsonResponse([]));

            await apiClient("/tickets");

            const init = fetchMock.mock.calls[0][1];
            expect(init?.method).toBeUndefined();
            expect(init?.body).toBeUndefined();
            expect(init?.headers).toEqual({
                "Content-Type": "application/json",
            });
        });

        it("forwards the method and the serialized body unchanged", async () => {
            const { apiClient } = await loadClient("https://api.example.com");
            fetchMock.mockResolvedValue(jsonResponse({ id: "1" }, 201));
            const body = JSON.stringify({ title: "Bug", description: "Broken" });

            await apiClient("/tickets", { method: "POST", body });

            const init = fetchMock.mock.calls[0][1];
            expect(init?.method).toBe("POST");
            expect(init?.body).toBe(body);
            expect(JSON.parse(init?.body as string)).toEqual({
                title: "Bug",
                description: "Broken",
            });
        });

        it("merges caller headers over the defaults", async () => {
            const { apiClient } = await loadClient("https://api.example.com");
            fetchMock.mockResolvedValue(jsonResponse({}));

            await apiClient("/tickets", {
                headers: {
                    "Content-Type": "text/plain",
                    "X-Request-Id": "abc",
                },
            });

            expect(fetchMock.mock.calls[0][1]?.headers).toEqual({
                "Content-Type": "text/plain",
                "X-Request-Id": "abc",
            });
        });
    });

    describe("successful responses", () => {
        it("returns the parsed JSON body", async () => {
            const { apiClient } = await loadClient("https://api.example.com");
            const stats = { open_tickets: 3, high_priority: 1, ai_analyzed: 2 };
            fetchMock.mockResolvedValue(jsonResponse(stats));

            await expect(apiClient("/dashboard/stats")).resolves.toEqual(stats);
        });

        it("returns undefined for 204 No Content without reading a body", async () => {
            const { apiClient } = await loadClient("https://api.example.com");
            const response = new Response(null, { status: 204 });
            const jsonSpy = vi.spyOn(response, "json");
            fetchMock.mockResolvedValue(response);

            await expect(
                apiClient("/tickets/1", {
                    method: "PATCH",
                    body: JSON.stringify({ status: "resolved" }),
                }),
            ).resolves.toBeUndefined();
            expect(jsonSpy).not.toHaveBeenCalled();
        });
    });

    describe("error responses", () => {
        it("throws an ApiError with the API message and status", async () => {
            const { apiClient, ApiError } = await loadClient(
                "https://api.example.com",
            );
            fetchMock.mockResolvedValue(
                jsonResponse({ error: "ticket not found" }, 404),
            );

            const error = await apiClient("/tickets/missing").catch(
                (err: unknown) => err,
            );

            expect(error).toBeInstanceOf(ApiError);
            expect(error).toBeInstanceOf(Error);
            expect(error).toMatchObject({
                name: "ApiError",
                message: "ticket not found",
                status: 404,
            });
        });

        it.each([
            ["a non-JSON body", () => new Response("Bad Gateway", { status: 502 })],
            ["an empty body", () => new Response(null, { status: 500 })],
            [
                "a non-string error field",
                () => jsonResponse({ error: { code: 42 } }, 400),
            ],
        ])(
            "uses a generic message when the response has %s",
            async (_, makeResponse) => {
                const { apiClient, ApiError } = await loadClient(
                    "https://api.example.com",
                );
                const response = makeResponse();
                fetchMock.mockResolvedValue(response);

                const error = await apiClient("/tickets").catch(
                    (err: unknown) => err,
                );

                expect(error).toBeInstanceOf(ApiError);
                expect(error).toMatchObject({
                    message: `API request failed with status ${response.status}`,
                    status: response.status,
                });
            },
        );
    });

    describe("network failures", () => {
        it("propagates the fetch error without wrapping it in ApiError", async () => {
            const { apiClient, ApiError } = await loadClient(
                "https://api.example.com",
            );
            const networkError = new TypeError("fetch failed");
            fetchMock.mockRejectedValue(networkError);

            const error = await apiClient("/tickets").catch(
                (err: unknown) => err,
            );

            expect(error).toBe(networkError);
            expect(error).not.toBeInstanceOf(ApiError);
        });

        it("propagates a caller abort without treating it as a timeout", async () => {
            const { apiClient, ApiTimeoutError } = await loadClient(
                "https://api.example.com",
            );
            const abortError = new DOMException("Aborted", "AbortError");
            fetchMock.mockRejectedValue(abortError);

            const error = await apiClient("/tickets").catch(
                (err: unknown) => err,
            );

            expect(error).toBe(abortError);
            expect(error).not.toBeInstanceOf(ApiTimeoutError);
        });
    });

    describe("timeouts", () => {
        it("aborts requests after the default timeout", async () => {
            const { apiClient, DEFAULT_TIMEOUT_MS } = await loadClient(
                "https://api.example.com",
            );
            const timeoutSpy = vi.spyOn(AbortSignal, "timeout");
            fetchMock.mockResolvedValue(jsonResponse([]));

            await apiClient("/tickets");

            expect(DEFAULT_TIMEOUT_MS).toBe(15_000);
            expect(timeoutSpy).toHaveBeenCalledExactlyOnceWith(15_000);
            expect(fetchMock.mock.calls[0][1]?.signal).toBe(
                timeoutSpy.mock.results[0].value,
            );
        });

        it("uses a custom timeout without forwarding it to fetch", async () => {
            const { apiClient } = await loadClient("https://api.example.com");
            const timeoutSpy = vi.spyOn(AbortSignal, "timeout");
            fetchMock.mockResolvedValue(jsonResponse({}));

            await apiClient("/tickets/1/triage", {
                method: "POST",
                timeoutMs: 75_000,
            });

            expect(timeoutSpy).toHaveBeenCalledExactlyOnceWith(75_000);
            expect(fetchMock.mock.calls[0][1]).not.toHaveProperty("timeoutMs");
        });

        it("combines a caller signal with the timeout", async () => {
            const { apiClient } = await loadClient("https://api.example.com");
            const controller = new AbortController();
            fetchMock.mockResolvedValue(jsonResponse([]));

            await apiClient("/tickets", { signal: controller.signal });

            const signal = fetchMock.mock.calls[0][1]?.signal;
            expect(signal).toBeInstanceOf(AbortSignal);
            expect(signal).not.toBe(controller.signal);
            expect(signal?.aborted).toBe(false);

            controller.abort();

            expect(signal?.aborted).toBe(true);
        });

        it("throws an ApiTimeoutError when the request times out", async () => {
            const { apiClient, ApiError, ApiTimeoutError } = await loadClient(
                "https://api.example.com",
            );
            fetchMock.mockRejectedValue(
                new DOMException("The operation timed out.", "TimeoutError"),
            );

            const error = await apiClient("/tickets").catch(
                (err: unknown) => err,
            );

            expect(error).toBeInstanceOf(ApiTimeoutError);
            expect(error).not.toBeInstanceOf(ApiError);
            expect(error).toMatchObject({
                name: "ApiTimeoutError",
                timeoutMs: 15_000,
            });
        });

        it("throws an ApiTimeoutError when reading the body times out", async () => {
            const { apiClient, ApiTimeoutError } = await loadClient(
                "https://api.example.com",
            );
            const response = jsonResponse([]);
            vi.spyOn(response, "json").mockRejectedValue(
                new DOMException("The operation timed out.", "TimeoutError"),
            );
            fetchMock.mockResolvedValue(response);

            await expect(apiClient("/tickets")).rejects.toBeInstanceOf(
                ApiTimeoutError,
            );
        });

        it("aborts a real request once the timeout fires", async () => {
            const { apiClient, ApiTimeoutError } = await loadClient(
                "https://api.example.com",
            );
            // Behave like fetch: reject with the signal's reason on abort.
            fetchMock.mockImplementation(
                (_, init) =>
                    new Promise((_, reject) => {
                        init?.signal?.addEventListener("abort", () =>
                            reject(init.signal?.reason),
                        );
                    }),
            );

            await expect(
                apiClient("/tickets", { timeoutMs: 10 }),
            ).rejects.toBeInstanceOf(ApiTimeoutError);
        });
    });
});

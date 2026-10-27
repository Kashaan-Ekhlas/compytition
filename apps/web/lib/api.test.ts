import { afterEach, describe, expect, it, vi } from "vitest";
import { api, ApiError, errorMessage, fieldErrors } from "./api";

function mockFetch(response: Response) {
  const spy = vi.fn().mockResolvedValue(response);
  vi.stubGlobal("fetch", spy);
  return spy;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("api", () => {
  it("builds query strings and keeps credentials on the request", async () => {
    const spy = mockFetch(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const result = await api<{ ok: boolean }>("/events", {
      query: { q: "hack", status: "", limit: 10, offset: undefined },
    });

    expect(result).toEqual({ ok: true });
    const [url, init] = spy.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/events?q=hack&limit=10");
    expect(init.credentials).toBe("include");
    expect((init.headers as Record<string, string>).Accept).toBe("application/json");
    expect((init.headers as Record<string, string>)["Content-Type"]).toBeUndefined();
    expect(init.body).toBeUndefined();
  });

  it("serialises JSON bodies and sets the content type", async () => {
    const spy = mockFetch(new Response(null, { status: 204 }));

    await api("/auth/login", { method: "POST", body: { email: "a@example.com" } });

    const [, init] = spy.mock.calls[0] as [string, RequestInit];
    expect(init.body).toBe(JSON.stringify({ email: "a@example.com" }));
    expect((init.headers as Record<string, string>)["Content-Type"]).toBe("application/json");
  });

  it("returns undefined for 204 responses", async () => {
    mockFetch(new Response(null, { status: 204 }));
    await expect(api("/events/abc", { method: "DELETE" })).resolves.toBeUndefined();
  });

  it("throws ApiError with the server envelope", async () => {
    mockFetch(
      new Response(
        JSON.stringify({
          error: {
            code: "validation_failed",
            message: "Some fields need attention.",
            fields: { email: "Already registered." },
          },
        }),
        { status: 422, headers: { "Content-Type": "application/json" } },
      ),
    );

    const error = await api("/auth/register", { method: "POST", body: {} }).catch((err) => err);

    expect(error).toBeInstanceOf(ApiError);
    const apiError = error as ApiError;
    expect(apiError.status).toBe(422);
    expect(apiError.code).toBe("validation_failed");
    expect(apiError.message).toBe("Some fields need attention.");
    expect(apiError.fields).toEqual({ email: "Already registered." });
    expect(fieldErrors(apiError)).toEqual({ email: "Already registered." });
    expect(errorMessage(apiError)).toBe("Some fields need attention.");
  });

  it("falls back to a generic error for non-JSON responses", async () => {
    mockFetch(new Response("Bad gateway", { status: 502 }));

    const error = (await api("/events").catch((err) => err)) as ApiError;
    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(502);
    expect(error.code).toBe("request_failed");
    expect(error.message).toBe("Request failed (502).");
    expect(error.fields).toBeUndefined();
  });
});

describe("errorMessage / fieldErrors", () => {
  it("handles plain errors and unknown values", () => {
    expect(errorMessage(new Error("boom"))).toBe("boom");
    expect(errorMessage("weird")).toBe("Something went wrong.");
    expect(fieldErrors(new Error("boom"))).toEqual({});
  });
});

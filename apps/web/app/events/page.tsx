"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, errorMessage } from "@/lib/api";
import type { EventView } from "@/lib/types";
import { EventCard } from "@/components/events/event-card";
import { EmptyState } from "@/components/illustrations";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { useAuth } from "@/components/auth-provider";

const PAGE_SIZE = 12;

const formats = [
  { value: "", label: "Any format" },
  { value: "individual", label: "Solo" },
  { value: "team", label: "Team" },
  { value: "both", label: "Solo or team" },
];

const modes = [
  { value: "", label: "Anywhere" },
  { value: "online", label: "Remote / Online" },
  { value: "onsite", label: "In person" },
];

const statuses = [
  { value: "", label: "All" },
  { value: "upcoming", label: "Upcoming" },
  { value: "open", label: "Registration open" },
  { value: "closed", label: "Registration closed" },
  { value: "ended", label: "Ended" },
];

export default function EventsPage() {
  const { profile } = useAuth();
  const [events, setEvents] = useState<EventView[]>([]);
  const [categories, setCategories] = useState<string[]>([]);
  const [total, setTotal] = useState(0);
  const [query, setQuery] = useState("");
  const [debounced, setDebounced] = useState("");
  const [category, setCategory] = useState("");
  const [format, setFormat] = useState("");
  const [status, setStatus] = useState("");
  const [mode, setMode] = useState("");
  const [country, setCountry] = useState("");
  const [state, setState] = useState("");
  const [city, setCity] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const request = useRef(0);

  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(query.trim()), 300);
    return () => window.clearTimeout(timer);
  }, [query]);

  const load = useCallback(
    async (offset: number) => {
      const id = ++request.current;
      if (offset === 0) setLoading(true);
      else setLoadingMore(true);
      setError(null);
      try {
        const data = await api<{ events: EventView[]; total: number; categories: string[] }>("/events", {
          query: {
            q: debounced, category, format, status, mode, country, state, city,
            limit: PAGE_SIZE, offset,
          },
        });
        if (id !== request.current) return;
        setEvents((current) => (offset === 0 ? data.events : [...current, ...data.events]));
        setTotal(data.total);
        if (data.categories) setCategories(data.categories);
      } catch (err) {
        if (id === request.current) setError(errorMessage(err));
      } finally {
        if (id === request.current) {
          setLoading(false);
          setLoadingMore(false);
        }
      }
    },
    [debounced, category, format, status, mode, country, state, city],
  );

  useEffect(() => {
    void load(0);
  }, [load]);

  const activeFilters = useMemo(
    () => [category, format, status, debounced, mode, country, state, city].filter(Boolean).length,
    [category, format, status, debounced, mode, country, state, city],
  );

  return (
    <div className="mx-auto max-w-content px-4 py-10 sm:px-6">
      <header className="max-w-reading">
        <p className="font-pixel text-sm text-muted">DISCOVERY</p>
        <h1 className="mt-1 font-pixel text-3xl sm:text-4xl">FIND YOUR NEXT EVENT</h1>
        <p className="mt-2 text-lg text-muted">
          Open hackathons, design jams and competitions. Filter by what fits your weekend.
        </p>
      </header>

      <div className="mt-8 border-2 border-ink bg-surface p-4 shadow-hard sm:p-5">
        <Label htmlFor="search">Search</Label>
        <Input
          id="search"
          className="h-14 text-xl"
          placeholder="Search by name, topic or host…"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          type="search"
        />
        <div className="mt-4 flex flex-col gap-4 lg:flex-row lg:items-end">
          <div className="flex-1">
            <Label>Category</Label>
            <div className="flex flex-wrap gap-2">
              <FilterChip active={category === ""} onClick={() => setCategory("")}>
                All
              </FilterChip>
              {categories.map((item) => (
                <FilterChip key={item} active={category === item} onClick={() => setCategory(item)}>
                  {item}
                </FilterChip>
              ))}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3 lg:w-80">
            <div>
              <Label htmlFor="format">Format</Label>
              <Select id="format" value={format} onChange={(event) => setFormat(event.target.value)}>
                {formats.map((item) => (
                  <option key={item.value} value={item.value}>
                    {item.label}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <Label htmlFor="status">Status</Label>
              <Select id="status" value={status} onChange={(event) => setStatus(event.target.value)}>
                {statuses.map((item) => (
                  <option key={item.value} value={item.value}>
                    {item.label}
                  </option>
                ))}
              </Select>
            </div>
          </div>
        </div>
        <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <Label htmlFor="mode">Where</Label>
            <Select
              id="mode"
              value={mode}
              onChange={(event) => {
                const value = event.target.value;
                setMode(value);
                if (value === "online") {
                  setCountry("");
                  setState("");
                  setCity("");
                }
              }}
            >
              {modes.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label htmlFor="country">Country</Label>
            <Input
              id="country"
              value={country}
              onChange={(event) => setCountry(event.target.value)}
              placeholder="Any country"
              disabled={mode === "online"}
            />
          </div>
          <div>
            <Label htmlFor="state">State or region</Label>
            <Input
              id="state"
              value={state}
              onChange={(event) => setState(event.target.value)}
              placeholder="Any state"
              disabled={mode === "online"}
            />
          </div>
          <div>
            <Label htmlFor="city">City</Label>
            <Input
              id="city"
              value={city}
              onChange={(event) => setCity(event.target.value)}
              placeholder="Any city"
              disabled={mode === "online"}
            />
          </div>
        </div>
        {profile && (profile.city || profile.state || profile.country) ? (
          <div className="mt-3">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => {
                setMode("onsite");
                setCountry(profile.country ?? "");
                setState(profile.state ?? "");
                setCity(profile.city ?? "");
              }}
            >
              Use my location
            </Button>
            <span className="ml-3 font-mono text-sm text-muted">
              {[profile.city, profile.state, profile.country].filter(Boolean).join(", ")}
            </span>
          </div>
        ) : null}
        <div className="mt-3 flex items-center justify-between gap-2">
          <p className="font-mono text-sm text-muted">
            {loading ? "Searching…" : `${total} event${total === 1 ? "" : "s"}`}
            {activeFilters ? ` · ${activeFilters} filter${activeFilters === 1 ? "" : "s"}` : ""}
          </p>
          {activeFilters ? (
            <Button
              variant="link"
              size="sm"
              onClick={() => {
                setQuery("");
                setCategory("");
                setFormat("");
                setStatus("");
                setMode("");
                setCountry("");
                setState("");
                setCity("");
              }}
            >
              Clear filters
            </Button>
          ) : null}
        </div>
      </div>

      {error ? (
        <Alert tone="danger" className="mt-6">
          {error}
        </Alert>
      ) : null}

      {loading ? (
        <LoadingBlock label="Loading events" />
      ) : events.length === 0 ? (
        <div className="mt-8">
          <EmptyState
            title="NO EVENTS MATCH"
            action={
              <Button variant="secondary" onClick={() => { setQuery(""); setCategory(""); setFormat(""); setStatus(""); setMode(""); setCountry(""); setState(""); setCity(""); }}>
                Reset filters
              </Button>
            }
          >
            Try a broader search, or host the event you wish existed.
          </EmptyState>
        </div>
      ) : (
        <>
          <div className="mt-8 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {events.map((event) => (
              <EventCard key={event.id} event={event} />
            ))}
          </div>
          {events.length < total ? (
            <div className="mt-8 flex justify-center">
              <Button variant="secondary" onClick={() => void load(events.length)} disabled={loadingMore}>
                {loadingMore ? "Loading…" : `Load more (${total - events.length} left)`}
              </Button>
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "min-h-10 border-2 border-ink px-3 py-1 font-mono text-sm transition-transform active:translate-x-[2px] active:translate-y-[2px]",
        active ? "bg-accent shadow-hard-sm" : "bg-surface hover:bg-accent/20",
      )}
    >
      {children}
    </button>
  );
}

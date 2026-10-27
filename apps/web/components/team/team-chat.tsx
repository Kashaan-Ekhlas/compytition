"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api, errorMessage } from "@/lib/api";
import type { ChatMessage } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Alert } from "@/components/ui/feedback";
import { formatDate } from "@/lib/utils";

function wsUrl(teamId: string): string {
  const base = process.env.NEXT_PUBLIC_WS_BASE;
  const path = `/api/v1/teams/${teamId}/chat`;
  if (base) return `${base.replace(/\/$/, "")}${path}`;
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}${path}`;
}

export function TeamChat({ teamId, currentUserId }: { teamId: string; currentUserId: string }) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [body, setBody] = useState("");
  const [status, setStatus] = useState<"connecting" | "open" | "closed">("connecting");
  const [error, setError] = useState<string | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const retryRef = useRef(0);

  const scrollToBottom = useCallback(() => {
    const el = listRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, []);

  useEffect(() => {
    let cancelled = false;
    let retryTimer: number | undefined;

    async function loadHistory() {
      try {
        const data = await api<{ messages: ChatMessage[] }>(`/teams/${teamId}/messages`, {
          query: { limit: 50 },
        });
        if (!cancelled) setMessages(data.messages);
      } catch (err) {
        if (!cancelled) setError(errorMessage(err));
      }
    }

    function connect() {
      if (cancelled) return;
      setStatus("connecting");
      const socket = new WebSocket(wsUrl(teamId));
      socketRef.current = socket;
      socket.onopen = () => {
        retryRef.current = 0;
        setStatus("open");
      };
      socket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data) as
            | { type: "message"; message: ChatMessage }
            | { type: "error"; message: string }
            | { type: "pong" };
          if (data.type === "message") {
            setMessages((current) =>
              current.some((m) => m.id === data.message.id) ? current : [...current, data.message],
            );
          } else if (data.type === "error") {
            setError(data.message);
          }
        } catch {
          /* ignore malformed frames */
        }
      };
      socket.onclose = () => {
        if (cancelled) return;
        setStatus("closed");
        const delay = Math.min(1000 * 2 ** retryRef.current, 15000);
        retryRef.current += 1;
        retryTimer = window.setTimeout(connect, delay);
      };
      socket.onerror = () => socket.close();
    }

    void loadHistory();
    connect();
    return () => {
      cancelled = true;
      if (retryTimer) window.clearTimeout(retryTimer);
      socketRef.current?.close();
    };
  }, [teamId]);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  function send(event: React.FormEvent) {
    event.preventDefault();
    const text = body.trim();
    if (!text || !socketRef.current || socketRef.current.readyState !== WebSocket.OPEN) return;
    socketRef.current.send(JSON.stringify({ type: "message", body: text }));
    setBody("");
  }

  return (
    <div className="border-2 border-ink bg-surface shadow-hard">
      <div className="flex items-center justify-between border-b-2 border-ink bg-accent-3 px-4 py-2">
        <h3 className="font-pixel text-base">TEAM CHAT</h3>
        <span className="font-mono text-xs uppercase tracking-wide">
          {status === "open" ? "● connected" : status === "connecting" ? "○ connecting" : "○ reconnecting"}
        </span>
      </div>
      <div ref={listRef} className="h-72 space-y-3 overflow-y-auto p-4" aria-live="polite">
        {messages.length === 0 ? (
          <p className="text-base text-muted">No messages yet. Say hello.</p>
        ) : null}
        {messages.map((message) => {
          const mine = message.user_id === currentUserId;
          return (
            <div key={message.id} className={mine ? "text-right" : ""}>
              <p className="font-mono text-xs text-muted">
                {mine ? "you" : message.username} · {formatDate(message.created_at, true)}
              </p>
              <p
                className={`mt-1 inline-block max-w-[85%] whitespace-pre-wrap break-words border-2 border-ink px-3 py-2 text-base ${
                  mine ? "bg-accent" : "bg-bg"
                }`}
              >
                {message.body}
              </p>
            </div>
          );
        })}
      </div>
      {error ? (
        <div className="px-4 pb-2">
          <Alert tone="danger">{error}</Alert>
        </div>
      ) : null}
      <form onSubmit={send} className="flex gap-2 border-t-2 border-ink p-3">
        <label htmlFor="chat-body" className="sr-only">
          Message
        </label>
        <Input
          id="chat-body"
          value={body}
          onChange={(event) => setBody(event.target.value)}
          maxLength={2000}
          placeholder={status === "open" ? "Message your team…" : "Connecting…"}
          disabled={status !== "open"}
        />
        <Button type="submit" disabled={status !== "open" || !body.trim()}>
          Send
        </Button>
      </form>
    </div>
  );
}

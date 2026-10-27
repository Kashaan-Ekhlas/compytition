# Compytition Design System

**Vibe:** retro tech, very minimal, always readable. Think chunky old-computer stickers on a clean page. The retro flavor is seasoning; legibility is the meal.

Reference moods:
1. **Halftone / CRT collaboration landing page:** cream and pale-blue paper, thick black outlines, hard offset shadows, sticker-style label boxes, a yellow accent band, dot-pattern texture, a beige CRT monitor illustration.
2. **80s synthwave arcade page:** deep navy bar, pixel-font navigation, purple / magenta / cyan neon, pixel-art cabinets, highlighted text blocks.

We borrow: outlines, hard shadows, pixel headings, halftone dots in decorative areas, one bold accent color per screen.
We do NOT borrow: visual noise, busy backgrounds behind text, tiny pixel text, glow effects that hurt contrast.

---

## 1. Principles

1. **Readable first.** Body text is large. If retro styling and readability conflict, readability wins.
2. **Minimal.** One primary action per screen. Lots of whitespace. No decorative clutter.
3. **Retro in the details.** Pixel headings, hard borders, offset shadows, CRT-ish illustrations, subtle halftone texture.
4. **Fewest taps.** Big targets, obvious next step, keyboard shortcuts where useful (especially judging).
5. **Never decorate behind text.** Halftone and textures live in margins, hero backgrounds and empty states, never behind paragraphs or form inputs.

---

## 2. Color

Two themes. Light is the default ("paper"). Dark is the synthwave variant ("arcade").

### Light (paper)
| Token | Hex | Use |
|---|---|---|
| `--bg` | `#F4EFE3` | Page background (warm cream) |
| `--surface` | `#FFFFFF` | Cards, inputs |
| `--ink` | `#14110F` | Text, borders, shadows |
| `--muted` | `#5B5650` | Secondary text (meets AA on bg) |
| `--accent` | `#FFD83D` | Primary highlight (yellow band / buttons) |
| `--accent-2` | `#D9414E` | Emphasis, destructive, key words (red) |
| `--accent-3` | `#C7CCF2` | CRT-screen lavender-blue, secondary panels |
| `--success` | `#2F8F5B` | Success |
| `--focus` | `#3B2BFF` | Focus ring |

### Dark (arcade)
| Token | Hex | Use |
|---|---|---|
| `--bg` | `#120B2E` | Deep navy-purple |
| `--surface` | `#1C1342` | Cards |
| `--ink` | `#F6F1FF` | Text |
| `--muted` | `#B7AEDD` | Secondary text |
| `--accent` | `#FF4FD8` | Magenta primary |
| `--accent-2` | `#2EE6D6` | Cyan secondary |
| `--accent-3` | `#7A3FF2` | Purple panels |
| `--focus` | `#FFD83D` | Focus ring |

Rules:
- Text on any background must meet **WCAG AA** (4.5:1 body, 3:1 large). Verify, do not assume.
- Use **one accent color per screen** as the dominant highlight. Do not rainbow.
- Neon colors in dark mode are for borders, highlights, and large headings, not long body text.
- Never convey meaning by color alone (pair with icon or label).

---

## 3. Typography

| Role | Font | Notes |
|---|---|---|
| Display / headings | **Silkscreen** or **Press Start 2P** (via `next/font`) | Pixel font. Use ONLY for headings, nav labels, badges, and short labels. Keep headings short (under ~6 words). Never for paragraphs. |
| Body / UI | **Space Grotesk** or **IBM Plex Sans** | Clean, readable. This is 95% of the text. |
| Data / code / timestamps | **IBM Plex Mono** or **JetBrains Mono** | Scores, rankings, deadlines, usernames |

Scale (large by default):
- Base body: **18px** (never below 16px anywhere), line-height 1.6
- Small/meta: 15px minimum
- H3: 24px, H2: 32px, H1: 44px (mobile 32px), hero: 56-72px
- Pixel headings: use sizes that land on clean multiples (e.g. 16 / 24 / 32px) and generous letter-spacing; avoid sizes below 14px for pixel fonts.
- Max line length ~70 characters for paragraphs.

Highlight treatment (from the references): important words can sit in a **sticker label**: text on a solid accent background with a 2-3px ink border and hard offset shadow, or a "marker highlight" (accent background behind inline text). Use sparingly, one or two per screen.

---

## 4. Shape, Borders, Shadows

- **Borders:** 2px solid `--ink` on cards, inputs, buttons. 3px on hero sticker labels.
- **Radius:** small, `4px` (or 0 for pixel-sharp elements like badges). No big pill radii.
- **Shadows:** **hard, offset, no blur**: `4px 4px 0 var(--ink)` for cards and buttons, `2px 2px 0` for small elements. In dark mode use `4px 4px 0` in `--accent` or `--accent-3`.
- **Press state:** on press, shadow collapses and element translates (`translate(3px,3px)`, shadow `0`), like a physical key.
- **Halftone:** a subtle dot-pattern (CSS radial-gradient or SVG tile, ink at 8-12% opacity) allowed only on page margins, hero backgrounds, section dividers and empty states.
- **Pixel dividers:** dashed or stepped (pixel-stair) rules between sections.

---

## 5. Layout and Spacing

- 8px base grid. Generous padding: cards `24px`, sections `64px+` vertical on desktop.
- Content max-width ~1100px (reading areas ~720px).
- Top nav: simple bar, pixel-font labels (Home, Events, Dashboard, Host, Judge), ink border-bottom, sticky. Collapses to a menu button on mobile (square, chunky, hamburger).
- One column on mobile; two max on desktop for forms. Cards in a clear grid for discovery.
- Touch targets **at least 48x48px**.

---

## 6. Components (shadcn re-themed)

Restyle every shadcn component to these tokens. Do not ship defaults.

- **Button:** primary = `--accent` bg, ink text, 2px border, hard shadow, bold. Secondary = surface bg. Destructive = `--accent-2`. Min height 48px, text 18px.
- **Input / Textarea / Select:** 2px ink border, white surface, 18px text, label ALWAYS visible above the field (no placeholder-only labels), clear error text in `--accent-2` with an icon.
- **Card:** surface, 2px border, hard shadow. Optional header strip in `--accent-3` with pixel-font title (like a window title bar).
- **Window motif:** event cards and modals can use a tiny "title bar" with three square dots (old-OS window chrome). Keep it flat and tasteful.
- **Badge / Tag:** pixel font, square corners, accent bg (e.g. OPEN, CLOSING SOON, TEAM, SOLO).
- **Tabs / Nav:** pixel labels, active tab = accent underline block, not just a color change.
- **Dialog / Sheet:** window-chrome style, backdrop with subtle halftone dim.
- **Toasts:** sticker-style, top-right, readable, auto-dismiss with enough time (6s+), dismissible.
- **Progress:** chunky segmented bar (blocks), used in onboarding and judging progress.
- **Empty states:** small pixel-art illustration (CRT monitor, floppy disk, arcade cabinet) + one sentence + one button.
- **Loading:** blinking block cursor or segmented loader. Skeletons use flat blocks, no shimmer required.

---

## 7. Key Screens

### Onboarding (first screen)
- Big pixel headline (e.g. "COMPYTE. JUDGE. WIN." or a short pitch), a sentence of body text, a CRT-monitor illustration on the right (desktop) or above (mobile), yellow accent band at the bottom with halftone.
- Two large buttons: **Sign in with Google**, **Use email**. Plain text link: Browse events.
- Steps after: segmented progress bar, one question per screen, large inputs, back button always visible.

### Event Discovery
- Search bar prominent at top (large, 56px height). Filters as chunky toggle chips (Category, Format, Status).
- Event cards: title bar strip, big event name, date and deadline in mono, badges, one clear "View" action. Maximum info density that still reads easily: name, host, dates, format, status.

### Judging Screen (most important UI)
- Two submission panels side by side (stacked on mobile) with equal weight, labelled **A** and **B** in large pixel type.
- **Pinned criteria strip** across the top: short criterion labels as chips, always visible, never scrolls away.
- Two big "Pick A" / "Pick B" buttons, keyboard `1` / `2` or arrow keys, plus a smaller "Skip" and "Flag conflict".
- Auto-advance on pick, segmented progress ("34 / 60"), brief undo toast.
- No numeric inputs. No sliders. No weights.

### Dashboard
- Simple sections: My Events, Teams, Invitations, (Host / Judge sections appear by role). Each section a card with a title bar and at most 5 rows plus "See all".

---

## 8. Motion

- Minimal and snappy: 100-150ms transitions, step-based easing (`steps(3)`) for pixel feel on hover and press where it does not harm legibility.
- Optional tiny blinking cursor in headings or loaders.
- Respect `prefers-reduced-motion`: disable all non-essential animation.
- No parallax, no autoplaying loops that distract.

---

## 9. Iconography and Illustration

- Icons: chunky outlined set (e.g. Lucide at 2px+ stroke), sized 24px minimum. Pixel-art illustrations for empty states, onboarding and hero only; keep a consistent small palette from the tokens.
- Illustration subjects: CRT monitor, keyboard, floppy, arcade cabinet, cursor hand, stickers. Build as inline SVG or crisp PNG with `image-rendering: pixelated` for pixel art.

---

## 10. Accessibility Checklist

- AA contrast verified for all token pairs, in both themes.
- Visible focus ring: 3px `--focus` outline with 2px offset on every interactive element.
- Full keyboard operation; logical tab order; skip-to-content link.
- Semantic HTML, labelled inputs, ARIA only where needed.
- Text resizable to 200% without breaking layout.
- Do not rely on pixel fonts for critical long information; always keep a readable sans fallback for content.

---

## 11. Do / Don't

**Do:** big text, hard shadows, one accent, whitespace, sticker labels for emphasis, pixel fonts for short headings.
**Don't:** blur shadows, gradients behind text, glassmorphism, tiny text, more than one pixel font, halftone behind paragraphs, neon body text, clutter. (The whole point of Compytition is that other platforms are cluttered.)

---

## 12. Tailwind Token Sketch

```ts
// tailwind.config.ts (extend)
colors: {
  bg: "var(--bg)", surface: "var(--surface)", ink: "var(--ink)",
  muted: "var(--muted)", accent: "var(--accent)",
  "accent-2": "var(--accent-2)", "accent-3": "var(--accent-3)",
},
boxShadow: { hard: "4px 4px 0 var(--ink)", "hard-sm": "2px 2px 0 var(--ink)" },
borderRadius: { DEFAULT: "4px" },
fontFamily: {
  pixel: ["var(--font-pixel)", "monospace"],
  sans: ["var(--font-sans)", "system-ui", "sans-serif"],
  mono: ["var(--font-mono)", "ui-monospace", "monospace"],
},
```

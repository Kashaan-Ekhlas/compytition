# Compytition Design System

**Vibe:** deep arcade colour with chunky, readable type. Think an old-computer sticker sheet on a calm page: hard borders, offset shadows and one bright accent, never at the cost of legibility.

Reference moods:
1. **80s synthwave arcade page:** deep navy-purple background, magenta highlights, purple panels, pixel-art cabinets.
2. **Halftone / CRT collaboration landing page:** thick outlines, hard offset shadows, sticker-style labels, dot-pattern texture in decorative areas.

We borrow: outlines, hard shadows, halftone dots in decorative areas, one bold accent per screen, window-chrome details.
We do NOT borrow: a second light theme, pixel body text, glow effects that hurt contrast, visual noise behind forms.

---

## 1. Principles

1. **Readable first.** Body text is large. Headings use the same readable grotesk as buttons, not a pixel face.
2. **Minimal.** One primary action per screen. Lots of whitespace. No decorative clutter.
3. **Retro in the details.** Hard borders, offset shadows, CRT-ish illustrations, subtle halftone texture.
4. **Fewest taps.** Big targets, obvious next step, keyboard shortcuts where useful (especially judging).
5. **Never decorate behind text.** Halftone and textures live in margins, hero backgrounds and empty states, never behind paragraphs or form inputs.

---

## 2. Colour

There is **one theme only**: the arcade palette. There is no light mode, no theme switch and no `data-theme="paper"` branch. `html` always carries `data-theme="arcade"`.

| Token | Hex | Use |
|---|---|---|
| `--bg` | `#120B2E` | Page background (deep navy-purple) |
| `--surface` | `#1C1342` | Cards, inputs, menus |
| `--ink` | `#F6F1FF` | Text, borders, shadows |
| `--muted` | `#B7AEDD` | Secondary text |
| `--accent` | `#FF4FD8` | Magenta primary action and selected state |
| `--accent-2` | `#FF5D73` | **Red** emphasis, validation errors, destructive actions |
| `--accent-3` | `#7A3FF2` | Purple panels and title bars |
| `--success` | `#48D597` | Success |
| `--focus` | `#FFD83D` | Focus ring |

Each token also has an `--*-rgb` triplet (for example `--accent-rgb: 255 79 216`) so Tailwind can compose opacity modifiers such as `bg-accent/40`, `border-ink/20` and `bg-bg/95`.

Rules:
- Text on any background must meet **WCAG AA** (4.5:1 body, 3:1 large). Verify, do not assume.
- Use **one accent colour per screen** as the dominant highlight. Do not rainbow.
- Destructive controls are red (`--accent-2`), never cyan, blue or magenta. This includes remove, delete, revoke and dissolve actions.
- Never convey meaning by colour alone: pair it with an icon or label.
- Authentication failures use a prominent red toast (`--accent-2`) with a visible X close button.

---

## 3. Typography

| Role | Font | Notes |
|---|---|---|
| Display / headings | **Space Grotesk**, bold | Same readable face used for product headings such as "Find an event" and "Host an event". `.font-pixel` is a legacy class name that maps to this face. |
| Body / UI | **Space Grotesk** | 95% of the text. |
| Data / code / timestamps | **IBM Plex Mono** | Scores, rankings, deadlines, usernames |

Scale (large by default):
- Base body: **18px** (never below 16px anywhere), line-height 1.6
- Small/meta: 15px minimum
- H3: 24px, H2: 32px, H1: 44px (mobile 32px), hero: 56-72px
- Headings: tight leading (1.15-1.3), negative tracking only at hero sizes.
- Max line length ~70 characters for paragraphs.

Highlight treatment: important words can sit in a **sticker label**: text on a solid accent background with a 2-3px ink border and hard offset shadow. Use sparingly, one or two per screen, and avoid oversized boxes around single words in a stacked hero.

No pixel or Minecraft-style headings. The old Silkscreen / Press Start 2P direction is retired.

---

## 4. Shape, Borders, Shadows

- **Borders:** 2px solid `--ink` on cards, inputs, buttons. 3px on sticker labels.
- **Radius:** small, `4px` (or 0 for sharp badges). No big pill radii.
- **Shadows:** **hard, offset, no blur**: `4px 4px 0 var(--ink)` for cards and buttons, `2px 2px 0` for small elements. Accent shadows use `--accent`.
- **Press state:** on press, shadow collapses and element translates (`translate(3px,3px)`, shadow `0`), like a physical key.
- **Halftone:** a subtle dot pattern (CSS radial-gradient at 12% ink opacity) allowed only on page margins, hero backgrounds, section dividers and empty states.
- **Pixel dividers:** dashed or stepped rules between sections.

---

## 5. Layout and Spacing

- 8px base grid. Generous padding: cards `24px`, sections `64px+` vertical on desktop.
- Content max-width ~1100px (reading areas ~720px).
- Top nav: simple bar with readable labels (Events, Dashboard, Host, Judge), an ink border-bottom and sticky positioning. Dashboard, Host and Judge are **hidden while logged out**. Collapses to a menu button on mobile.
- Auth box: centred, generous width (about 560px), one clear heading and one primary action.
- One column on mobile; two max on desktop for forms. Cards in a clear grid for discovery.
- Touch targets **at least 48x48px**.

---

## 6. Components (shadcn re-themed)

Restyle every shadcn component to these tokens. Do not ship defaults.

- **Button:** primary = `--accent` bg, ink text, 2px border, hard shadow, bold. Secondary = surface bg. Destructive = `--accent-2` with white text. Min height 48px, text 18px.
- **Input / Textarea / Select:** 2px ink border, surface background, 18px text, label ALWAYS visible above the field (no placeholder-only labels), clear error text in `--accent-2` with an icon.
- **Password input:** every password field, including confirm password, has an eye toggle that switches between masked and visible text.
- **Password checklist:** five criteria (8+ characters, uppercase, lowercase, number, special symbol) each with a pink circular check. The register and join buttons stay disabled until all five pass and the confirmation matches.
- **Card:** surface, 2px border, hard shadow. Optional header strip in `--accent-3` with a readable title, like a window title bar.
- **Window motif:** event cards and modals can use a tiny title bar with three square dots. Keep it flat and tasteful.
- **Badge / Tag:** readable bold label, square corners, accent or panel background (OPEN, CLOSING SOON, TEAM, SOLO).
- **Tabs / Nav:** readable labels, active tab = accent underline block, not just a colour change.
- **Dialog / Sheet:** window-chrome style, backdrop with subtle halftone dim.
- **Toasts:** sticker-style, top-right, readable. Errors are red, include an X button and stay for 6 seconds. Toasts are used for wrong email/password instead of a quiet inline message.
- **Progress:** chunky segmented bar (blocks), used in onboarding and judging progress.
- **Empty states:** small illustration (CRT monitor, floppy disk, arcade cabinet) + one sentence + one button.
- **Loading:** segmented loader. Skeletons use flat blocks, no shimmer required.
- **Notifications:** the trigger is a **bell** icon with an unread count. It updates in real time (polling plus focus/visibility refresh) without opening the menu.
- **Footer:** compact. Feedback only, linking to the feedback section of settings. No Events or Host links.
- **Icons:** chunky outlined set (Lucide at 2px+ stroke), 24px minimum. Save and share icons on event cards and detail pages are at least 40px targets with visible labels.

---

## 7. Key Screens

### Landing
- Hero heading: **"A calm home for hackathons, design jams, and family-friends contests."**
- Stacked wordmark may read `COMPYTE. / COMPARE. / DECIDE.` with consistent alignment and no oversized pink box around a single word.
- No blinking cursor, no "JUDGE 1 / 2 / 3" stage labels on the public landing page.
- No "No ads. No entry fees. No scores out of ten." band.
- Logged-out nav shows only public links. Dashboard, Host and Judge appear after sign-in.

### Auth
- One sign-in card, centred and wider than before. Exactly one "Create an account" link.
- Supporting copy: "One account covers hosting, competing and judging."
- No "SMALL COMPARISONS. BIG SIGNAL." block.
- Wrong email/password shows a red dismissible toast.

### Onboarding
- Seven steps, segmented progress bar, one question per screen, large inputs, back button always visible.
- Username availability is accurate for the current account: an unchanged username is available, not "taken".
- College/company and course use curated suggestions plus free text.
- Optional country, state and city are collected **sequentially**: state appears after country, city after state.
- Intent cards choose whether the dashboard leans toward entering, hosting or judging.

### Event Discovery
- Search bar prominent at top (large, 56px height). Filters as chunky chips: category, format, status, mode (remote/online or in person) and location.
- "Use my location" fills the profile's country/state/city and switches the mode to in person.
- Switching to remote/online clears stale country/state/city fields and disables them.
- Event cards are spacious and informative: category and status, title, organizer username, description, dates, registration deadline, entry and registration counts, plus **save/bookmark** and **share** actions. The layout follows the supplied reference screenshot rather than a compressed tile.

### Hosting
- Event creation location starts with **Remote / Online** first, then country, state and city for in-person events.
- Contact email offers "Use mine" to copy the account email in one tap.
- Team size asks only for a maximum; there is no minimum team size.
- Organizers can merge registration and submission forms. When merged, registration includes submission fields. When separate, the submission form holds the extra questions.
- Judging settings are derived from submission and judge counts. Manual numeric judging configuration is not exposed, and judging cannot start before the submission deadline.

### Judging Screen (most important UI)
- Two submission panels side by side (stacked on mobile) with equal weight, labelled **A** and **B**.
- **Pinned criteria strip** across the top: short criterion labels as chips, always visible.
- Two big "Pick A" / "Pick B" buttons, keyboard `1` / `2` or arrow keys, plus a smaller "Skip" and "Flag conflict".
- Custom submission answers appear in each panel with their form labels.
- Auto-advance on pick, segmented progress ("34 / 60"), brief undo toast.
- No numeric inputs. No sliders. No weights.

### Dashboard
- Simple sections: My Events, Teams, Invitations, with host and judge sections appearing by role. Each section is a card with a title bar and at most 5 rows plus "See all".
- Bookmarks and invitation links are visible without digging into a menu.

---

## 8. Motion

- Minimal and snappy: 100-150ms transitions, step-based easing for a mechanical feel where it does not harm legibility.
- No blinking cursors in public headings.
- Respect `prefers-reduced-motion`: disable all non-essential animation.
- No parallax, no autoplaying loops that distract.

---

## 9. Iconography and Illustration

- Icons: chunky outlined set, 24px minimum. Bell for notifications, bookmark for save, share for sharing, eye for password visibility, red X for dismissal and removal.
- Illustration subjects: CRT monitor, keyboard, floppy, arcade cabinet, cursor hand, stickers. Build as inline SVG or crisp PNG.
- Illustrations support the copy; they never replace a text label.

---

## 10. Accessibility Checklist

- AA contrast verified for all token pairs in the single arcade theme.
- Visible focus ring: 3px `--focus` outline with 2px offset on every interactive element.
- Full keyboard operation; logical tab order; skip-to-content link.
- Semantic HTML, labelled inputs, ARIA only where needed.
- Toasts use `role="alert"` and an accessible dismiss label.
- Text resizable to 200% without breaking layout.
- Do not rely on decorative fonts for critical information.

---

## 11. Do / Don't

**Do:** big readable type, hard shadows, one accent, whitespace, sticker labels for emphasis, clear red destructive actions.
**Don't:** light mode, pixel headings, em dashes, blinking cursors, blur shadows, gradients behind text, glassmorphism, tiny text, halftone behind paragraphs, neon body text, clutter. (The whole point of Compytition is that other platforms are cluttered.)

---

## 12. Tailwind Token Sketch

```ts
// tailwind.config.mjs (extend)
colors: {
  bg: "rgb(var(--bg-rgb) / <alpha-value>)",
  surface: "rgb(var(--surface-rgb) / <alpha-value>)",
  ink: "rgb(var(--ink-rgb) / <alpha-value>)",
  muted: "rgb(var(--muted-rgb) / <alpha-value>)",
  accent: "rgb(var(--accent-rgb) / <alpha-value>)",
  "accent-2": "rgb(var(--accent-2-rgb) / <alpha-value>)",
  "accent-3": "rgb(var(--accent-3-rgb) / <alpha-value>)",
  success: "rgb(var(--success-rgb) / <alpha-value>)",
  focus: "rgb(var(--focus-rgb) / <alpha-value>)",
},
boxShadow: { hard: "4px 4px 0 var(--ink)", "hard-sm": "2px 2px 0 var(--ink)" },
borderRadius: { DEFAULT: "4px" },
fontFamily: {
  pixel: ["Space Grotesk", "ui-sans-serif", "system-ui", "sans-serif"],
  sans: ["Space Grotesk", "ui-sans-serif", "system-ui", "sans-serif"],
  mono: ["IBM Plex Mono", "ui-monospace", "monospace"],
},
```

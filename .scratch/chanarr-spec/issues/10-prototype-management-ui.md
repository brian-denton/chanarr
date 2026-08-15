# Prototype: channel management UI

Type: prototype
Status: resolved
Blocked by: 08, 09

## Question

What does the React UI look like for the north-star flow (point at folder → channel exists) plus channel editing (name/number/logo/shuffle)? Rough clickable prototype via /prototype to react to; resolves the onboarding flow and screen inventory for the spec.

## Prototype

Self-contained HTML/JS, no build step, three structurally different variants switchable via `?variant=A|B|C` or the floating bottom bar: [assets/ui-prototype/index.html](../assets/ui-prototype/index.html) — open directly in a browser.

- **A — Wizard-first**: full-screen step flow for onboarding (folder path → scan result → create), minimal single-column channel list afterward, full-screen edit panel. Plex-connect is a dismissible top banner.
- **B — Dashboard + slide-over**: home is a card grid (like a media-server dashboard); add-channel is an inline expanding card; editing opens a right-side slide-over. Plex-connect is a persistent status pill in the header.
- **C — Live guide grid**: home *is* an EPG-style channel × timeline grid (leans into "this is a real TV channel"); add-channel is a row at the bottom of the grid; editing opens a bottom drawer. Plex-connect is a top banner, dismissible, tied to the metaphor of "the guide isn't live yet."

Self-verified in-browser (Claude Browser pane): all three variants render, the add-channel flow (folder input → scan → create) works and updates the channel list/grid/guide correctly, the PIN-link Plex-connect flow displays a code and resolves to "connected" after a delay, and state persists correctly across variant switches. Caught and fixed one real bug in the process: the "Scan folder" button's disabled state wasn't updating as the user typed, because typing only patches state without a full re-render (to avoid stealing input focus) — fixed by directly toggling the button's `disabled` property from the input handler.

## Answer

User picked **Variant A — Wizard-first onboarding**: full-screen step flow (folder → scan result → create) for onboarding, a minimal single-column channel list as the home screen afterward, full-screen panel for channel editing, dismissible top banner for the optional Plex-connect prompt. This is the structural direction for chanarr's real UI — it leans hardest into the no-docs, one-thing-at-a-time north star among the three.

Additional requirement surfaced during this exchange: **the UI must support both light and dark mode.** Default: follow OS/browser system preference, with a manual override toggle (not investigated further — a standard, low-risk pattern; no open sub-question worth its own ticket).

Repo has no git history yet (not a git repository as of charting), so the full three-variant set stays as the primary source at [assets/ui-prototype/index.html](../assets/ui-prototype/index.html) rather than a throwaway branch — Variants B and C are kept there for reference/salvage, not folded into the real implementation.

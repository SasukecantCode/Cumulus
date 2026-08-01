# Cumulus web

React 19 + Vite + Tailwind v4 front end for the Cumulus storage orchestrator.

```bash
npm install
npm run dev     # http://localhost:3000, proxies /api /auth /workspaces to :8080
npm run build
npm run lint
```

## The sky system

The entire interface is themed by **one custom property: `--sky-t`** (registered
via `@property`, range 0 → 1).

| value | mood | when |
| ----- | ---- | ---- |
| `0` | clear morning sky, soft white cumulus | landing page, signed out |
| `1` | heavy overcast sky | workspace, ~120s after signing in |

It is a colour drift and nothing else — no rain, no weather effects. The only
motion in the backdrop is the cloud field, which drifts at the same slow pace
whatever the colour.

`src/index.css` defines every token twice — a `--c-*` clear value and a `--s-*`
storm value — and resolves each one with `color-mix(… var(--mix))`. Because
`--sky-t` is a registered property it can be *transitioned*, so moving that
single number drifts every surface, shadow, label and icon together. Nothing
else needs a slow transition.

`src/theme/SkyProvider.jsx` owns the dial:

- `rollIn()` — signed in; 120s drift toward overcast (called by `Dashboard`)
- `clearUp()` — signed out; 9s back to blue (called by `LandingPage`)

`src/components/SkyBackdrop.jsx` draws the fixed atmosphere behind everything:
a sun that fades out as the sky darkens, and a field of drifting cumulus.
Note that the sky gradient must always land on the panel colour at the bottom
of the page — neumorphic surfaces only read as moulded while they share the
background they sit on.

## Building UI

Surfaces are neumorphic: a panel is the *same colour* as its background and is
defined purely by light and shadow. Use the primitives in
`src/components/ui/Soft.jsx` (`Panel`, `SoftButton`, `IconButton`, `Meter`,
`Chip`, `Segmented`) and the classes they wrap — `nm`, `nm-sm`, `nm-lg`,
`nm-inset`, `nm-lift`, `nm-btn`, `nm-pressed`, `nm-input`.

Colour tokens are exposed to Tailwind through `@theme`, so `text-ink`,
`text-soft`, `hover:text-accent`, `text-warn` and `text-good` are real
utilities with full variant support. Never hard-code a hex value in a
component — it will not follow the weather.

Structure and rhythm come from four classes: `.shell` (the single gutter
authority — every screen's content sits in one), `.section-title`, `.label`
and `.well`. Use them instead of re-deriving spacing per screen.

Overlays all go through `src/components/Modal.jsx` (`variant="dialog"` or
`"drawer"`), which owns the scrim, Escape handling and header. File kinds
(icon, `--tint-*` colour, category filter) live in `src/lib/files.js`.

```
src/components/
  workspace/     Rail, PageHeader, StorageOverview, AccountsPanel,
                 FileBrowser, FileGlyph — the signed-in dashboard
  ui/            Soft (Panel, SoftButton, IconButton, Meter, Chip,
                 Segmented), Ring
  *.jsx          shells and overlays: Dashboard, LandingPage, Modal,
                 AccountsDrawer, UploadModal, FileDetailModal
```

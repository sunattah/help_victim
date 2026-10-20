# Victim Care — Product Blueprint

> **Working title:** Victim Care
> **Tagline:** *Care when it matters most.*
> **Document status:** v1.0 — implementation blueprint
> **Owner:** help_victim / victimcare
> **Last updated:** 2026-10-20

---

## 1. Vision

Victim Care is a safety companion for people who are injured, threatened, or in
danger. It keeps the three things that matter in an emergency **one tap away**:

1. **Raise the alarm** — a 5-second cancelable SOS that prepares a call, a text, and a location share.
2. **Do the right thing first** — plain-language first-aid steps for the most common emergencies.
3. **Tell someone you trust** — emergency contacts that can be reached from the device in one tap.

The product is deliberately **assisted** rather than fully automatic. The app never
sends a message or places a call on its own; it *prepares* the alert and hands
the final send to the user. This keeps the user in control, avoids false alarms,
and works on any phone without requiring carrier integration or a server-side
telephony account.

### Non-goals (v1)

- Not a replacement for emergency services.
- Not a monitoring/tracking service (no background location, no live tracking).
- Not a mental-health or counselling product.
- Not a public social feed.

---

## 2. Problem statement

| Problem | Who feels it | Current work-around |
|---|---|---|
| Under pressure, people cannot remember emergency numbers or what to do first | Victim, bystander | Frozen; search a web page at the wrong moment |
| Contact details for help are scattered across apps and phone books | Victim | Fumbling for a contact while hurt or afraid |
| First-aid advice online is long, ad-heavy, and rarely matches the emergency | Victim, bystander | Abandoning the page to search again |
| Panic buttons on other apps either fire instantly (false alarms) or take over the phone | Victim | Not trusting the button at all |
| Privacy: many apps demand location and contacts "just to work" | Victim | Refusing to install |

**Solution:** one screen, one big button, five seconds of grace, and everything
else on the same phone that is already in their hand.

---

## 3. Users and personas

**Priya — the directly affected victim**
Mid-30s, walking home alone, phone at 30% battery. Needs: one button, a number
that works in her country, and instructions she can follow while shaking.
Success = alert prepared in < 10 seconds.

**Marcus — the friend/bystander**
At the scene of a fall. Needs: what to do *right now* (bleeding, choking, breathing).
Success = correct first step in < 30 seconds, no jargon.

**Dr. Amaka — the support worker / counsellor**
Helps victims plan for safety. Needs: a contact list the user keeps current and a
report history to review with them.
Success = the user comes back with contacts filled in and settings tuned.

**Adaeze — the organizational buyer (NGO, campus safety, clinic)**
Needs: a deployable, low-cost, offline-capable safety card; usage stats that are
aggregate and privacy-preserving; no PII in analytics.
Success = deploys to 500+ students in a day.

---

## 4. Core user journeys

### J1 — SOS (the critical path)
1. User opens the app → signed in → Home.
2. Taps the big **SOS** button.
3. Full-screen red overlay appears, counting down **5 → 4 → 3 → 2 → 1**.
4. **Cancel** stops everything and returns to Home. (No state is mutated on cancel.)
5. On zero, the app resolves location (8 s timeout, graceful fallback) and shows the
   **Alert ready** state.
6. Ready state offers: **Call `<emergency number>`**, **Text `<contact>`** (with a
   pre-written message containing name, incident type, notes, and a maps link), and
   **Close**.
7. The alert is recorded in **Past reports** with a timestamp.

### J2 — Report a problem
Home → Report → pick a type (Accident / Medical / Assault / Fire / Other) →
write what happened → **Prepare alert** → same location + ready flow as J1 (no countdown).

### J3 — First aid
Home → First aid → expandable cards: Heavy bleeding, Burns, Choking, Not breathing,
Broken bone. Each is 3–4 numbered steps in imperative voice. No wall of text.

### J4 — Contacts
Home → Contacts → save one **primary emergency contact** (name + phone).
The contact's number powers the "Text my contact" action in every alert.

### J5 — Settings (see §6.4 — full spec)
Home → Settings → five sub-sections: **Profile**, **Appearance**, **Alerts**,
**Account**, **About**.

### J6 — Session lifecycle
Welcome → Log in / Sign up → app. **Sign out** (header button and in Settings →
Account) clears the session, wipes in-memory state, and returns to Welcome.

---

## 5. Information architecture / screen map

```
Welcome (signed out)
├── Log in (name + email, prototype auth)
└── Sign up (first name, second name, email, state, nation, password, confirm)
        │
        ▼
App shell  [header: brand · + Create · Sign out]  [nav: Home · Report · First aid · Contacts · Settings]
├── Home
│    ├── SOS button (5 s countdown)
│    ├── Tile: Report a problem
│    ├── Tile: First aid
│    ├── Tile: Contacts
│    └── Tile: Settings
├── Report
│    ├── Type chips (Accident, Medical, Assault, Fire, Other)
│    ├── What happened? (textarea)
│    ├── Prepare alert
│    └── Past reports (history list)
├── First aid
│    └── 5 expandable guides
├── Contacts
│    ├── Primary contact form
│    └── Call emergency services (tel: link)
└── Settings
     ├── Profile      → edit name, email, state, nation, avatar upload
     ├── Appearance   → theme picker (4 themes) + preview
     ├── Alerts       → emergency number, share location, vibrate
     ├── Account      → change password, sign out
     └── About        → version, credits, links, safety disclaimer
```

---

## 6. Screen specifications

### 6.0 App shell

- **Header** (red, sticky): brand mark + "Victim Care" wordmark; right side holds
  **＋ Create** and **Sign out** (visible only when signed in).
- **Nav bar**: five equal tabs, active tab marked with `aria-current="page"` and a
  3 px red underline. Collapses to smaller type below 650 px.
- **Content width**: max 1100 px, 22 px side gutters (15 px on mobile).
- **Panel resize control**: header slider (85 %–120 %) that scales the app shell
  for demoing on desktop; **Reset size** restores 100 %. Purely cosmetic.

### 6.1 Welcome

- Split hero: **picture area** (left/right, see §7) + copy (right).
- Copy: eyebrow "HELP STARTS HERE", H2 "Care when it matters most.", supporting line,
  **Log in** (solid red) and **Sign up** (outline) buttons, and a prototype disclaimer.
- The **picture area** shows a rotating set of images with a slow drift/zoom
  animation (§7).

### 6.2 Auth

- Single panel reused for both modes.
- **Log in**: Email, Password. **Sign up**: First name, Second name, Email,
  **Country of residence (dropdown)**, **State / Region (dropdown)**, Nation of
  origin, Password, Confirm password (+ Show/Hide toggles).
- **Country comes before state.** The country is a dropdown with a visible
  arrow; choosing it fills the state dropdown with the states, provinces or
  regions of that country. Countries without a known list get a free-text
  fallback ("Other (not listed)").
- **The email must be an email address and nothing else.** The server rejects
  addresses without a top level domain, with spaces, with a missing or
  repeated `@`, with empty or consecutive dots, or with a numeric top level
  domain. The message is *"Enter a valid email address, for example
  name@example.com."*
- **The password must contain letters and numbers.** At least 8 characters, at
  least one letter and at least one digit. When it fails the error reads
  *"Password is incorrect. It must contain at least one number."* The signup
  form shows the three rules as a live checklist with a strength meter.
- Client-side validation with an inline `role="alert"` error region; the server
  re-validates everything.
- **Back to welcome** returns without side effects.

### 6.3 Home

- Intro card: "Do you need help?" + "Press SOS to prepare an emergency alert.
  You have 5 seconds to cancel. Calls and texts must be sent from your device."
- **SOS**: 150 px circular red button with a pulsing halo, label "SOS", helper
  "Press for help". Animation respects `prefers-reduced-motion`.
- **Practise the alert**: a dashed button under the SOS button that runs the
  same countdown and shows the same prepared alert, but sends nothing and
  saves nothing. It is there so a user can learn the flow before they need it.
- Four tiles in a 2×2 grid (1 column below 365 px): Report, First aid, Contacts, Settings.

### 6.4 Settings — *the section the user asked for*

Settings is a **sectioned page** inside the app shell (reached from the nav tab
and from the Home tile). It is one scrollable page with five clearly separated
cards, so nothing is buried in a sub-menu.

#### 6.4.1 Profile — *edit profile & upload pictures*

| Field | Type | Rules |
|---|---|---|
| First name | text | required, ≤ 80 chars |
| Second name | text | required, ≤ 80 chars |
| Email address | email | required, unique, ≤ 160 chars |
| State of residence | text | required, ≤ 80 chars |
| Nation of origin | text | required, ≤ 80 chars |
| Current password | password | required only when changing the email |

- **Pictures**: the user can upload **several** pictures. Each upload is
  validated by sniffing its real content type (not the browser supplied one),
  re-encoded, resized to a max 1024 px master plus a 256 × 256 square
  thumbnail, and stored **outside** the web root with a random file name. The
  browser never receives a filesystem path.
- Accepted formats: **JPG and PNG** (decoded with the Go standard library).
  Maximum **5 MB**, minimum **128 × 128 px**.
- The gallery in Settings shows every uploaded picture with a thumbnail. Any
  picture can be marked as the **profile picture** (shown in the header) or
  deleted. The header avatar falls back to the user's initials.
- Clicking a picture (or the profile preview) opens a **full-screen lightbox**.
- Removing the profile picture promotes the next picture automatically.

#### 6.4.2 Appearance — *change themes of the app*

A **theme picker**: five live-preview swatches. Selecting a theme re-themes the
**entire** app instantly (header, nav, cards, forms, overlay, SOS button) by
swapping CSS custom properties — no page reload.

| Theme | Name | Palette |
|---|---|---|
| `light` | **Light** (default) | red `#d62828`, deep `#9d0208`, blush `#fff0f0`, ink `#3a1012` |
| `dark` | **Dark** | red `#ff6b6b`, deep `#ffb3b3`, surface `#17151a`, ink `#f6f1f1` |
| `calm` | **Calm** | teal `#0f766e`, deep `#115e59`, surface `#f0fdfa`, ink `#134e4a` |
| `contrast` | **High contrast** | black and white with a `#ffe600` accent, heavier borders, no tints |
| `auto` | **Auto (system)** | follows `prefers-color-scheme` |

- Each swatch shows the theme name and its three colour dots, and is a real
  radio control (`role="radio"`, `aria-checked`, arrow-key navigable) so it
  works with a keyboard and a screen reader.
- The chosen theme is stored **server-side on the user profile** *and* mirrored to
  `localStorage`. The page reads that mirror in an inline script before the
  first paint, so the app never flashes the wrong colours.
- A **reduced motion** switch ("Animate pictures and effects") lives here as
  well. It switches off the picture animation, the cross-fade and the SOS pulse.

#### 6.4.3 Alerts

- **Emergency number to call** (default `112`, free-text; the app stores the number
  used by every "Call" link).
- **Share my location in alerts** (checkbox, default on).
- **Vibrate when an alert is prepared** (checkbox, default on).
- **Panic countdown seconds** (3/5/10, default 5) — how long the user has to cancel.
- Saves with a visible status line ("Settings updated") and applies immediately.

#### 6.4.4 Account

- **Change password** (current + new + confirm) — real bcrypt re-hash.
- **Sign out** — a destructive-styled button; clears the session cookie, the
  in-memory state and the `localStorage` theme mirror, then returns to Welcome.
  Sign-out is also available from the header.
- **Delete account** — asks for the password to confirm, removes the profile,
  the avatar and the user's reports. This is destructive and irreversible, so it
  requires explicit confirmation.

#### 6.4.5 About

- App name, version, build date, and a one-paragraph description of what the app
  does and explicitly what it does **not** do.
- **Safety notice**: "Victim Care does not replace emergency services. If you are in
  immediate danger, call your local emergency number."
- Credits (first-aid content reviewers, icon set, font licence).
- Links: Terms, Privacy, Accessibility statement.
- A short **"How your data is used"** list: password hashing, avatar storage,
  no third-party analytics, no selling of data.

### 6.5 Report

- Type chips (single select, `aria-pressed`), free-text "What happened?" with a
  helpful placeholder, **Prepare alert**, and a **Past reports** history list
  (most recent first, capped at 20) with type, note, and timestamp.
- **Clear past reports** lives in Settings → Account (keeps destructive actions in
  one place).

### 6.6 First aid

Five `<details>` cards, each a short numbered list. Content is versioned and
reviewed by a qualified first-aider before release.

### 6.7 Contacts

- The user can save **several** contacts, not just one.
- Every contact shows its name and phone number. One of them is the
  **primary** contact; the text alert goes to the primary contact. The first
  contact a user saves becomes the primary one automatically.
- Each contact can be edited, deleted, or promoted to primary. Deleting the
  primary contact promotes another one.
- **Add a contact** form with name and phone (E.164 hint, `+234…`).
- **Call emergency services** → `tel:` link using the number from Settings.

### 6.8 Alert overlay

Three states, one overlay, `role="alertdialog"`, `aria-modal="true"`:

1. **Countdown** — "Preparing alert in", a huge number, **Cancel**.
2. **Location** — "Getting your location…" while geolocation resolves.
3. **Ready** — "Alert ready", a line explaining nothing has been sent yet, then
   **Call `<number>`**, **Text `<primary contact>`** (hidden when no contact is
   set, with a "Add an emergency contact in Contacts" message), **Copy the
   message** (puts the full alert text on the clipboard so it can be pasted
   into any messenger), and **Close**.

A **practice run** shows the same three states but changes the heading to
"Practice run finished", says that nothing was sent or saved, and records no
report.

---

## 7. The picture area (animated)

The Welcome hero contains a **picture area** holding a small gallery of images.
The pictures **move**:

- **Continuous drift (Ken Burns)** — each visible image slowly zooms (1.0 → 1.12)
  and pans across the frame on a slow loop. The zoom/pan direction alternates per
  slide so consecutive pictures never move identically.
- **Cross-fade transitions** — outgoing picture fades and scales slightly down
  while the incoming one fades in; the images are stacked in the same grid cell.
- **Auto-advance** — every 6 s the gallery moves to the next picture.
- **Dot indicators** — one dot per picture; the active dot is elongated and
  clickable; clicking a dot jumps straight to that picture.
- **Pause on hover/focus** — the animation stops while the pointer is over the
  picture area or it has keyboard focus, so it never distracts or moves under
  the user's cursor. It also stops when the browser tab is hidden.
- **Reduced motion** — when `prefers-reduced-motion: reduce` is set (or the user
  turns off motion in Settings), the drift, the cross-fade and the auto-advance all
  stop; the pictures remain visible as a static, gently cross-faded set.
- **Implementation**: pure CSS (`@keyframes` on `transform: scale()/translate()`)
  plus a tiny JS controller for slide state and auto-advance. No image library.
- **Content**: five **figurative** slides — drawn scenes with recognisable
  people and objects rather than abstract shapes:
  1. *Care* — a paramedic kneeling beside a person on the ground, holding a
     bandage, with a first aid bag and a city skyline behind them.
  2. *First aid* — an open first aid case with a white cross, a rolled
     bandage, a plaster and a glove.
  3. *Support* — one person with an arm around another, under a shared heart.
  4. *Response* — an ambulance with a red cross and a blue light bar, driving
     through a city at night.
  5. *Hope* — a lit house, a sunrise and a person standing safe next to a
     growing plant.

  The artwork ships with the app as SVG in `static/img/slides/`, so the hero
  works with no network at all. Each slide carries descriptive `alt` text.

---

## 8. Design system

### 8.1 Tokens (CSS custom properties)

```
--red      #d62828   primary / brand
--deep     #9d0208   headings, emphasis
--blush    #fff0f0   surface tint
--line     #f3c9c9   borders
--ink      #3a1012   body text
--muted    #75494b   secondary text
```

Every token is redefined per theme, so theming is a single class/attribute swap.

### 8.2 Typography

- Font: **Nunito** (Google Fonts) — weights 400 / 600 / 800.
- H1 21 px / H2 24 px / H3 19 px / body 16 px / small 13 px / hero 36 px.
- Line-height 1.5, headings 1.15.

### 8.3 Components

`.brand-header`, `.top-nav`, `.welcome`, `.form-panel`, `.content-panel`,
`.intro-card`, `.tile`, `.chip`, `.action` (+ `.outline`), `.sos`, `.report-card`,
`.setting-row`, `.theme-swatch`, `.avatar`, `.alert-overlay`, `.status`.

### 8.4 Breakpoints

- ≤ 650 px — single-column welcome, stacked settings rows, smaller nav type.
- ≤ 365 px — tiles collapse to one column, nav type shrinks further.

### 8.5 Accessibility

- Visible 3 px focus ring on every interactive element (`:focus-visible`).
- Landmarks: `header`, `nav`, `main`; live regions for status messages and the
  countdown (`aria-live="assertive"`).
- Colour contrast ≥ 4.5:1 for text in every theme (high-contrast theme ≥ 7:1).
- Full keyboard operation, including the theme radio group.
- All decorative motion disabled under `prefers-reduced-motion`.

---

## 9. Data model

```
users
  id             INTEGER PK
  first_name     TEXT
  second_name    TEXT
  email          TEXT UNIQUE
  password_hash  TEXT        (bcrypt, cost 12)
  country        TEXT        country of residence, from the country list
  state          TEXT        state / province / region of that country
  nation         TEXT        nation of origin
  theme          TEXT        light | dark | calm | contrast | auto
  motion         INTEGER     1 = animation allowed, 0 = reduced
  emergency_no   TEXT        default '112'
  share_location INTEGER     default 1
  vibrate        INTEGER     default 1
  countdown_secs INTEGER     default 5 (3 | 5 | 10)
  created_at     TIMESTAMP
  updated_at     TIMESTAMP

sessions
  token_hash TEXT PK   (SHA-256 of the cookie value, never the value itself)
  user_id    INTEGER FK
  csrf       TEXT
  expires_at TIMESTAMP
  created_at TIMESTAMP

pictures
  id         INTEGER PK
  user_id    INTEGER FK
  file       TEXT        random file name of the master image
  thumb      TEXT        random file name of the 256 px square thumbnail
  is_primary INTEGER     exactly one picture per user can be the profile picture
  created_at TIMESTAMP

contacts
  id         INTEGER PK
  user_id    INTEGER FK
  name       TEXT
  phone      TEXT
  is_primary INTEGER     the contact that text alerts go to
  created_at TIMESTAMP

incidents
  id              INTEGER PK
  user_id         INTEGER FK
  kind            TEXT     Accident | Medical | Assault | Fire | Other | SOS emergency
  note            TEXT
  latitude        REAL NULL
  longitude       REAL NULL
  location_shared INTEGER
  created_at      TIMESTAMP

countries (reference data, embedded in the binary)
  name     TEXT
  regions  TEXT[]   states / provinces / regions; empty when the country has
                     no known list and accepts a free-text region
```

---

## 10. API

All JSON endpoints live under `/api/v1`. Mutating requests need the
`X-CSRF-Token` header. Errors return `{"error": "...", "field": "..."}` with a
sensible status code.

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/v1/health` | no | Liveness + version |
| POST | `/api/v1/auth/signup` | no | Create account, start session |
| POST | `/api/v1/auth/login` | no | Start session |
| POST | `/api/v1/auth/logout` | yes | End session |
| GET | `/api/v1/auth/me` | yes | Current profile, CSRF token, pictures |
| PATCH | `/api/v1/users/me` | yes | Update name/email/state/nation |
| GET | `/api/v1/users/me/pictures` | yes | List the uploaded pictures |
| POST | `/api/v1/users/me/pictures` | yes | Multipart picture upload (`file`) |
| DELETE | `/api/v1/users/me/pictures/{id}` | yes | Delete a picture |
| POST | `/api/v1/users/me/pictures/{id}/primary` | yes | Choose the profile picture |
| GET | `/api/v1/users/me/settings` | yes | Theme + alert preferences |
| PUT | `/api/v1/users/me/settings` | yes | Save preferences |
| POST | `/api/v1/users/me/password` | yes | Change password |
| DELETE | `/api/v1/users/me` | yes | Delete account (needs password) |
| GET | `/api/v1/contacts` | yes | List contacts |
| POST | `/api/v1/contacts` | yes | Add a contact, or update one when an `id` is sent |
| DELETE | `/api/v1/contacts/{id}` | yes | Remove a contact (another one becomes primary) |
| POST | `/api/v1/contacts/{id}/primary` | yes | Choose the contact that text alerts go to |
| GET | `/api/v1/countries` | no | The country and state list used by the forms |
| GET | `/api/v1/incidents` | yes | Past reports |
| POST | `/api/v1/incidents` | yes | Record a prepared alert |
| DELETE | `/api/v1/incidents` | yes | Clear history |
| GET | `/api/v1/guides` | no | First-aid content |
| GET | `/media/picture/{id}` · `/media/picture/{id}/thumb` | yes | Serve an uploaded picture (owner only) |

The UI is server-rendered and progressively enhanced: the HTML page works with
JavaScript disabled for reading and for auth, while the interactive flows
(SOS countdown, theme preview, avatar preview) require JS. All data mutations go
through the JSON API.

---

## 11. Technical architecture

```
help_victim/
├── blueprint.md              ← this document
├── README.md
├── victimcare/               ← Go application
│   ├── go.mod
│   ├── main.go               ← entry point, graceful shutdown
│   ├── internal/
│   │   ├── config/           ← env + defaults
│   │   ├── models/           ← User, Picture, Contact, Incident
│   │   ├── countries/        ← embedded country and state list
│   │   ├── store/            ← SQLite (SQL) + migrations
│   │   ├── auth/             ← bcrypt, sessions, CSRF, cookies
│   │   ├── handlers/         ← HTTP handlers (pages + JSON API)
│   │   ├── imaging/          ← image validation + resize
│   │   └── guides/           ← first-aid content
│   ├── templates/            ← html/template pages
│   └── static/               ← css, js, images, manifest, service worker
└── data/                     ← runtime: victimcare.db + uploads/ (git-ignored)
```

- **Language / runtime:** Go 1.22, standard library first.
- **Routing:** `net/http` with Go 1.22 method-and-wildcard patterns
  (`GET /api/v1/incidents/{id}`) — no external router needed.
- **Templates:** `html/template` with layout partials; user data is escaped by default.
- **Database:** SQLite via `modernc.org/sqlite` (pure Go, no cgo) with embedded
  migrations, WAL mode, and `PRAGMA foreign_keys=ON`.
- **Auth:** `golang.org/x/crypto/bcrypt` (cost 12) + opaque random session
  tokens in an `HttpOnly`, `SameSite=Lax`, `Secure`-when-HTTPS cookie.
- **Uploads:** validated by sniffing magic bytes, re-encoded to JPEG/PNG/WebP,
  resized with `image/draw` + a simple fit, stored under `data/uploads/` with
  random names, and served only through an authenticated handler.
- **Configuration:** environment variables (`VICTIMCARE_ADDR`, `VICTIMCARE_DB`,
  `VICTIMCARE_UPLOADS`, `VICTIMCARE_SECRET`) with sensible defaults.

### Security rules

- Passwords never logged; session tokens hashed at rest.
- CSRF token required on every state-changing request.
- Uploaded files are never executed or served with their original name.
- Rate-limit auth endpoints (token bucket per IP).
- Security headers: `X-Content-Type-Options`, `X-Frame-Options: DENY`,
  `Referrer-Policy: no-referrer`, and a strict `Content-Security-Policy`.

---

## 12. Non-functional requirements

| Area | Requirement |
|---|---|
| Speed | First paint < 1 s on 3G; SOS overlay interactive < 100 ms |
| Offline | First-aid guides and the app shell are cached; the app degrades to read-only offline |
| Battery | No background polling; geolocation only on demand and only while the overlay is open |
| Privacy | No third-party analytics, no advertising, no sale of data; location never leaves the device except inside a message the user sends |
| Resilience | A failed geolocation request never blocks the alert — the flow continues with "location not shared" |
| Accessibility | WCAG 2.1 AA; keyboard-complete; reduced-motion honoured; screen-reader labels on all icon buttons |
| Portability | Single binary + one SQLite file; runs on a laptop, a Raspberry Pi, or a phone |
| Safety | The app never auto-dials; every send requires an explicit tap |

---

## 13. Safety and ethical considerations

This product is used by people in distress. Requirements that are **not negotiable**:

1. **No automatic dispatch.** The app prepares; the person sends.
2. **Never blame the user.** Copy is supportive and factual ("You are not in trouble").
3. **Escalation path is always visible.** The emergency number is one tap from
   every screen, including inside the alert overlay.
4. **Content review.** First-aid steps are written and signed off by a qualified
   first-aider; the About screen names the reviewer.
5. **Data minimisation.** Only the data the features need is collected, and the
   About screen tells the user exactly what is stored.
6. **No dark patterns.** The SOS button cannot be mis-pressed into an irreversible
   action — the countdown always runs first.

---

## 14. Milestones

| # | Milestone | Contents | Status |
|---|---|---|---|
| M1 | Blueprint | This document, design tokens, data model, API | ✅ |
| M2 | App shell | Go server, layout, routing, SQLite bootstrap, migrations | ✅ |
| M3 | Auth | Signup, login, logout, sessions, CSRF, password change | ✅ |
| M4 | Core flows | SOS countdown, report flow, geolocation, ready state, history | ✅ |
| M5 | Settings | Profile, picture uploads, themes, alerts, account, about | ✅ |
| M6 | Animated picture area | Ken Burns drift, cross-fade, dots, pause, reduced motion | ✅ |
| M7 | Contacts + guides | Primary contact, tel: links, first-aid content | ✅ |
| M8 | Hardening | Security headers, rate limits, tests, offline cache | 🟡 tests and headers done; rate limiting still to come |
| M9 | Upgrade round | Figurative artwork, country → state dropdowns, strict email, password rules, practice run, multiple contacts, installable app | ✅ |

---

## 15. Testing strategy

- **Unit** — validators (email, phone, password rules), theme tokens, image
  sniffing, countdown state machine.
- **Handler** — `httptest` round-trips for every API route: signup → login →
  profile update → avatar upload → settings save → incident create → logout.
- **Integration** — full signed-in journey with a temporary database.
- **Manual** — 360 px / 768 px / 1440 px, keyboard-only pass, reduced-motion pass,
  high-contrast theme pass.

---

## 16. Open questions

1. Should the emergency number be a per-country picker (using the user's `state`
   and `nation`) instead of a free-text field? *(v1: free text, default `112`.)*
2. Should multiple emergency contacts be supported in v1, or is one primary enough?
3. Do we want a "shared check-in" feature (auto-alert if the user misses a check-in)?
   Deferred — it needs a server and background scheduling.
4. What is the retention policy for incident history and avatars?

---

*Victim Care — care when it matters most.*

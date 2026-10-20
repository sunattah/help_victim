# Victim Care

**Care when it matters most.**

Victim Care is a safety companion for people who are injured, threatened or in
danger. It keeps three things one tap away:

1. **Raise the alarm** — a cancelable SOS that prepares a call, a text and a
   location share from your own device.
2. **Do the right thing first** — plain-language first-aid steps for the most
   common emergencies.
3. **Stay in control** — an editable profile, your own pictures, four themes
   plus a system theme, and an emergency contact you can reach in one tap.

The app never places a call or sends a message on its own. It *prepares* the
alert and hands the final send to you.

The full product design lives in [`blueprint.md`](blueprint.md).

---

## Running it

Requirements: **Go 1.22+** (no cgo, no external services).

```bash
cd victimcare
go mod download
go run .
```

Then open <http://localhost:8080>.

The first run creates `data/victimcare.db` (SQLite) and `data/uploads/`
next to the repository root.

### Configuration

| Variable | Default | Purpose |
|---|---|---|
| `VICTIMCARE_ADDR` | `:8080` | Listen address |
| `VICTIMCARE_DB` | `<root>/data/victimcare.db` | SQLite database file |
| `VICTIMCARE_UPLOADS` | `<root>/data/uploads` | Uploaded pictures |
| `VICTIMCARE_SECRET` | development value | Used to derive session and CSRF tokens — **change this in production** |
| `VICTIMCARE_SECURE_COOKIES` | `false` | Set to `true` when serving over HTTPS |
| `VICTIMCARE_MAX_UPLOAD_BYTES` | `5242880` | Maximum picture size (5 MB) |
| `VICTIMCARE_SESSION_HOURS` | `336` | Session lifetime |

---

## What's inside

```
help_victim/
├── blueprint.md              ← the product blueprint (design, data model, API)
├── README.md
├── Victim Care.html          ← the original click-through prototype
└── victimcare/               ← the Go application
    ├── main.go               ← entry point, graceful shutdown
    ├── internal/
    │   ├── config/           ← environment configuration
    │   ├── models/           ← User, Picture, Contact, Incident
    │   ├── countries/        ← embedded country + state list
    │   ├── store/            ← SQLite persistence + migrations
    │   ├── auth/             ← bcrypt, sessions, CSRF, cookies
    │   ├── handlers/         ← pages, JSON API, media routes
    │   ├── imaging/          ← upload validation, resize, thumbnail
    │   └── guides/           ← first-aid content
    ├── templates/index.html  ← server rendered app shell
    └── static/
        ├── css/app.css       ← design tokens + five themes
        ├── js/app.js         ← SOS countdown, settings, picture gallery
        ├── manifest.webmanifest ← installable app
        ├── sw.js             ← offline cache for the static shell
        └── img/              ← figurative artwork for the moving picture area
```

---

## The screens

| Screen | What it does |
|---|---|
| **Welcome** | Hero with the moving picture area, log in and sign up |
| **Home** | Big SOS button (5 s cancel window), a **practise the alert** button, tiles for report, first aid, contacts and settings |
| **Report** | Problem type, description, prepared alerts and the report history |
| **First aid** | Six expandable guides: bleeding, burns, choking, not breathing, broken bone, shock |
| **Contacts** | Several emergency contacts, one of them primary, plus a one tap call to the emergency number |
| **Settings** | Profile, pictures, appearance, alerts, account and about |

### Signing up

The sign up form asks for your **first name**, **second name**, **email
address**, **country of residence** and **state or region** — both as
dropdowns with an arrow, with the country first — then your **nation of
origin**, a **password** and a **confirmation**.

- **The email must be an email address and nothing else.** `amaka`, `amaka@`,
  `amaka@example`, `amaka example.com` and `amaka@example.123` are all
  refused.
- **The password must contain letters and numbers.** At least 8 characters,
  at least one letter and at least one digit. The three rules are shown as a
  live checklist with a strength meter, and a wrong password is reported as
  *"Password is incorrect. It must contain at least one number."*
- Picking a country fills the state dropdown with the states of that country.
  Countries without a known list let you type the region instead.

### Settings in detail

- **Profile** — edit first name, second name, email, country, state and nation.
  Changing the email address requires the current password.
- **Your pictures** — upload several pictures (JPG or PNG, up to 5 MB, at least
  128 × 128). Mark any of them as your profile picture, delete any of them, or
  open one in the full screen preview. The header avatar follows the profile
  picture, and falls back to your initials.
- **Appearance** — five themes: **Light**, **Dark**, **Calm**, **High contrast**
  and **Auto (system)**. The whole app re-themes instantly, the choice is saved
  to your profile and mirrored to `localStorage` so there is no flash of the
  wrong colours. An **animate pictures and effects** switch turns all motion off.
- **Alerts** — the emergency number to call, the seconds you get to cancel the
  SOS (3, 5 or 10), whether your location is shared and whether the phone
  vibrates.
- **Account** — change your password, clear your past reports, log out, or
  delete your account.
- **About** — what the app does, what it does not do, the safety notice, how
  your data is used, credits, the legal links and an **install** button that
  adds the app to your home screen.

### The moving picture area

The welcome hero holds a gallery of five **figurative** drawings — a paramedic
attending to someone, an open first aid kit, two people supporting each other,
an ambulance at night and a person standing safe at sunrise. The pictures
**move**: each slide drifts and zooms slowly (a Ken Burns effect), slides
cross-fade into each other, the gallery advances every six seconds and the dots
below it let you jump to a picture. The motion pauses while your pointer or
keyboard focus is on the gallery, and it is switched off completely by
`prefers-reduced-motion` or by the motion switch in Settings.

### Install it on a phone

Open the app in a browser and use **Settings → About → Install Victim Care**
(or the browser menu, "Add to home screen"). The app then runs standalone with
its own icon, and the static shell keeps working when the network is weak.

---

## API

All endpoints live under `/api/v1` and speak JSON. Mutating requests need the
`X-CSRF-Token` header (the token is handed to the page in the bootstrap block
and in the `victimcare_csrf` cookie).

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/health` | Liveness and version |
| POST | `/api/v1/auth/signup` · `/api/v1/auth/login` · `/api/v1/auth/logout` | Sessions |
| GET | `/api/v1/auth/me` | Current profile, CSRF token and pictures |
| PATCH | `/api/v1/users/me` | Edit the profile |
| GET/POST | `/api/v1/users/me/pictures` | List and upload pictures |
| DELETE | `/api/v1/users/me/pictures/{id}` | Delete a picture |
| POST | `/api/v1/users/me/pictures/{id}/primary` | Choose the profile picture |
| GET/PUT | `/api/v1/users/me/settings` | Theme and alert preferences |
| POST | `/api/v1/users/me/password` | Change the password |
| DELETE | `/api/v1/users/me` | Delete the account |
| GET/POST | `/api/v1/contacts` | Emergency contacts (add or update) |
| DELETE | `/api/v1/contacts/{id}` | Remove a contact |
| POST | `/api/v1/contacts/{id}/primary` | Choose the contact for text alerts |
| GET | `/api/v1/countries` | Country and state list |
| GET/POST/DELETE | `/api/v1/incidents` | Prepared alerts and history |
| GET | `/api/v1/guides` | First-aid content |

---

## Tests

```bash
cd victimcare
go test ./...
```

The suite covers the sign up → log in → profile → settings → contact → report →
picture upload → log out journey, CSRF enforcement, upload validation
(non-images, tiny images, oversized files), the email rules, the password
rules, the country/state pairing and the multiple contact flow.

---

## Safety notes

- Victim Care does **not** replace emergency services.
- The app never calls or texts automatically; every send needs your tap.
- Location stays on your device until you share it in a message.
- Passwords are hashed with bcrypt, session tokens are hashed at rest, and
  uploaded files are validated by their real content type.

---

*Victim Care — care when it matters most.*

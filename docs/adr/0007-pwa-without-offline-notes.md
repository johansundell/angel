# Installable App That Never Keeps Notes Offline

## Context
Angel can be installed to the home screen as a Progressive Web App, for the Client and for Caregivers who visit regularly. A service worker could cache the Daily Note so it can be read without signal. But a Daily Note can carry medical and routine details about the Client, and Caregivers read it on their own phones behind one shared Caregiver PIN (ADR-0004, ADR-0005). A cached note would stay on a phone after the Caregiver leaves and after their session ends, readable by anyone holding that phone without entering the PIN.

## Decision
The service worker caches only the app shell (stylesheet, icons, an offline page). Pages and fragments behind a PIN are always fetched from the network and never stored; offline, the app shows a Swedish "no connection" page instead of the note. This matches the existing `Cache-Control: no-store` on every PIN-protected page. One installable app opens at the PIN keypad, which already sends each PIN to its own view. Push notifications are out of scope.

## Considered Options
- **Cache notes everywhere for offline reading**: Caregivers could read the note without signal, but notes would outlive the session on devices the Client doesn't control, bypassing the Caregiver PIN.
- **Cache notes only on the Client's device**: The Client already wrote the note and gains little; the service worker would have to tell roles apart, and a mistake leaks notes to Caregiver phones.

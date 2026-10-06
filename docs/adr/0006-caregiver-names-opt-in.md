# Caregivers See Each Other's Acknowledgements; Names Are Opt-In

## Context
Caregivers visiting the Client's home could not tell whether anyone had already been there that Day; only the Client saw the list of today's Acknowledgements. Showing Caregivers that list helps them coordinate visits, and names help them hand over to the previous visitor. But the Caregiver view is behind one shared 4-digit Caregiver PIN, and ADR-0005 accepts that someone with many addresses can guess it. Whatever the Caregiver view shows, anyone who knows or guesses the PIN sees too: names and times together tell them who visits the Client, a vulnerable person, and when.

## Decision
The Caregiver view always shows the times of today's Acknowledgements, newest first. First names are shown with them only when `SHARE_CAREGIVER_NAMES=true` is set; it is off by default, and a value that isn't a boolean stops the service at startup. The setting lives in `.env`, like the Caregiver PIN, and changes with a restart. The Client dashboard says which of the two Caregivers see.

## Reasons
- **Coordination works everywhere**: Times alone answer "has anyone been here today, and when?" and reveal only that someone visited.
- **Sharing names is a deliberate choice**: Whoever sets up the service decides, knowing the PIN is shared; nothing personal is exposed by default, and a typo can't quietly share names.
- **No new storage**: A setting in `.env` needs nothing new in the SQLite, MySQL and FileMaker stores, and matches how the Caregiver PIN it protects is changed.
- **The Client stays informed**: The dashboard hint tells the Client what Caregivers see without reading the server's configuration.

## Considered Options
- **Always show names**: Simplest, but tells anyone with the PIN who visits and when.
- **Never show times or names**: Caregivers keep asking each other, or the Client, whether anyone has been there.
- **A toggle on the Client dashboard**: Lets the Client decide without a restart, but needs new storage in all three backends and a new kind of change from the dashboard; it can come later if the Client needs it.

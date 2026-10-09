# Warn the Client About PIN Guessing Instead of Capping Attempts Overall

## Context
Failed PINs are rate limited per client address: 5 wrong attempts every 15 minutes. An attacker with many addresses can still work through the 10,000 possible Caregiver PINs; with 1,000 addresses that is 5,000 guesses every 15 minutes. An overall cap would stop that, but any shared limit, whether it blocks or delays, is used up by the attacker first, so real Caregivers would be locked out of the day's note.

## Decision
We accept the risk of distributed guessing and make it visible instead. When 20 checked wrong PINs are entered within 24 hours, from any addresses, the Client dashboard shows a PIN Alert asking them to change the Caregiver PIN, and the service logs it. The alert is kept in memory and stays until the service restarts, which is also what changing the Caregiver PIN requires. Attempts rejected by the rate limiter without checking the PIN do not count.

Once triggered, the alert keeps counting every further checked wrong PIN, with no window, and moves its "last" time on, so the Client can see whether guessing is still going on. Only the title of the alert is a screen reader alert (`role="alert"`); while the dashboard is open, polling updates only the count and times below it, so the alert is announced once when it appears, not on every new wrong PIN.

## Reasons
- **Caregiver access comes first**: A Caregiver who can't read the day's instructions is a real harm to the Client; a distributed attack on one household's PIN is unlikely.
- **The fix belongs to the Client**: Only the Client can change the Caregiver PIN, so the alert goes to them.
- **No new storage**: An in-memory alert clears exactly when the PIN is changed.
- **Ongoing guessing stays visible without nagging**: A frozen count would hide whether the attack continues; announcing every increment would make the dashboard unusable with a screen reader.

## Considered Options
- **Overall hard cap**: Anyone could lock out every Caregiver.
- **Overall cap that delays instead of blocking**: Caregivers queue behind the attacker's requests, so it is a lockout in practice.
- **Freeze the count when the alert triggers**: Simpler, but the Client could not tell whether guessing had stopped.
- **Keep counting and announce every change**: Screen readers would interrupt the Client on every poll while guessing goes on.

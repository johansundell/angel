# Angel

A focused daily messaging interface enabling a home care client to communicate daily notes and instructions to visiting caregivers.

## Language

**Client**:
The person receiving home care who authors daily instructions and monitors acknowledgements.
_Avoid_: Admin, patient, customer, user

**Caregiver**:
A visiting home care professional who accesses the board on-site to read daily instructions.
_Avoid_: Personnel, staff, worker, employee

**Daily Note**:
An informational note or set of instructions published by the Client for a specific calendar date.
_Avoid_: Message, post, bulletin, announcement, ticket

**Acknowledgement**:
A single-action confirmation recorded by a Caregiver indicating that today's Daily Note has been read, optionally carrying a Caregiver first name and timestamp.
_Avoid_: Read receipt, check-in, signature, reaction

**Important Flag**:
A visual status applied to a Daily Note to emphasize critical routine or medical changes to visiting Caregivers.
_Avoid_: Priority, alert level, urgent message

**Advance Note**:
A Daily Note drafted ahead of time for tomorrow, activating automatically at the daily rollover.
_Avoid_: Scheduled post, draft, future note

**Rollover**:
The daily transition time (default midnight) when the current day advances and the next Daily Note becomes active.
_Avoid_: Expiration, reset, turnover

**Caregiver PIN**:
A shared 4-digit numeric code entered by visiting Caregivers to view the current day's note.
_Avoid_: Password, door code, passcode, visitor code

**Master PIN**:
A private numeric code entered by the Client to open the note authoring and management view.
_Avoid_: Admin password, master key, credentials

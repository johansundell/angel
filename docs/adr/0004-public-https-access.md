# Public HTTPS Access for Caregiver Mobile Devices

## Context
Visiting home care personnel need to access Angel upon arrival by scanning a QR code or visiting a URL. We considered restricting access to the local home Wi-Fi network versus exposing the service over public HTTPS.

## Decision
Angel will be served over public HTTPS (via Cloudflare Tunnel, VPS, or reverse proxy), allowing caregivers to access the site directly on cellular data (4G/5G).

## Reasons
- **Zero Wi-Fi friction**: Visiting staff do not need to be onboarded to or remember credentials for a home Wi-Fi network.
- **Immediate availability**: Scanning a QR code immediately opens the PIN keypad on any visiting caregiver's phone.
- **Security model**: The shared 4-digit Caregiver PIN protects note contents, and rate limiting / session timeouts prevent unauthorized brute-forcing.

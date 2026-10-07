# Angel behind a reverse proxy

Example configurations for running Angel behind **nginx** or **Apache**, with HTTPS from Let's Encrypt (ADR-0004). Each folder has:

| File | What it is |
|---|---|
| `angel-proxy.conf` | The proxy rules both setups share: forwarding headers and security headers. |
| `angel-http.conf` | Port 80 for production: answers Let's Encrypt challenges and redirects everything else to HTTPS. |
| `angel.conf` | Port 443 for production: TLS and the proxy to Angel. Lines marked `CHANGE` need your values. |
| `demo.conf`, `compose.yml` | A local demo on `http://localhost:8080` that uses the same proxy rules. |

`check.sh` tests a running setup through the proxy (see [Checking a setup](#checking-a-setup)).

For a server of its own, the repository also has a ready Docker Compose stack with Caddy, which gets and renews the certificate by itself: see [Running with HTTPS (Let's Encrypt)](../../README.md#running-with-https-lets-encrypt).

## What Angel needs from the proxy

- **A domain or subdomain of its own.** Angel uses absolute paths (`/assets/...`, `/pin`, `/admin`), so `https://angel.example.com/` works and `https://example.com/angel/` doesn't.
- **HTTPS.** The session cookie is `Secure`, so browsers only send it over HTTPS (or to `localhost`). Leave `COOKIE_SECURE` at its default `true`.
- **The client's address in `X-Forwarded-For`, from a trusted address.** Angel limits wrong PINs per client address: 5 wrong PINs per address in 15 minutes. Behind a proxy, every request comes from the proxy, so Angel reads the client's address from `X-Forwarded-For`, but only when the request comes from an address in `TRUSTED_PROXIES`.
  - If `TRUSTED_PROXIES` is empty or wrong, every visitor shares the proxy's address. Five wrong guesses from anyone then lock out every Caregiver for 15 minutes.
  - If the proxy passes on an `X-Forwarded-For` the client made up, anyone can choose a new address for every guess. The configs here replace the header with the address the proxy saw.
- **Nothing else in front of Angel.** Make Angel listen only where the proxy can reach it, so nobody can go around the proxy.

## Settings for Angel

With Angel and the proxy on the same host (the setup in `angel.conf`), put this in Angel's `.env`:

```bash
PORT="127.0.0.1:8080"         # only the proxy on this host can connect
TRUSTED_PROXIES="127.0.0.1"   # believe X-Forwarded-For only from the proxy
CAREGIVER_PIN="..."           # exactly 4 digits
MASTER_PIN="..."              # 4-12 digits, not the Caregiver PIN
SESSION_SECRET="..."          # at least 32 characters, e.g. from: openssl rand -base64 33
```

Without a fixed `SESSION_SECRET`, everyone has to enter the PIN again after Angel restarts.

`GET /logs` is reachable through the proxy too. It needs `AUTH_TOKEN`, and Caregivers never use it, so you can also block `/logs` in the proxy if you like.

### Angel in Docker, proxy on the host

This setup is easy to get wrong. Publish the port on localhost only (`ports: ["127.0.0.1:8080:8080"]`) and point the proxy at `127.0.0.1:8080`. Requests then reach Angel from the Docker network's **gateway**, not from `127.0.0.1`, so `TRUSTED_PROXIES` must be the gateway's address. Look it up with:

```bash
docker network inspect <project>_default -f '{{(index .IPAM.Config 0).Gateway}}'
```

Inside the container Angel always listens on `:8080`; don't set `PORT` there.

## Setting up HTTPS with Let's Encrypt

These steps are for Debian or Ubuntu, with `angel.example.com` standing for your host name. They use certbot's *webroot* method: certbot puts a file in `/var/www/letsencrypt`, and Let's Encrypt fetches it over port 80 through `angel-http.conf`. The proxy keeps running during renewals.

The HTTPS config points at certificate files that only exist once certbot has run, so the order matters: port 80 first, then the certificate, then HTTPS.

**1. DNS and firewall.** Point an `A` (and `AAAA`, if the server has IPv6) record for `angel.example.com` at the server, and open ports 80 and 443. Let's Encrypt must reach port 80 from the internet.

**2. Angel.** Install Angel with the `.env` from [Settings for Angel](#settings-for-angel) and start it, for example as a system service: `sudo ./angel -service install` then `sudo ./angel -service start`. Check it with `curl http://127.0.0.1:8080/healthz`.

**3. Proxy and certbot.**

```bash
# nginx
sudo apt install nginx certbot
# or Apache
sudo apt install apache2 certbot

sudo mkdir -p /var/www/letsencrypt
```

**4. Port 80.** Install `angel-http.conf` and replace `angel.example.com` in it.

```bash
# nginx
sudo cp nginx/angel-http.conf /etc/nginx/conf.d/angel-http.conf
sudo nginx -t && sudo systemctl reload nginx

# Apache
sudo cp apache/angel-http.conf /etc/apache2/sites-available/angel-http.conf
sudo a2ensite angel-http
sudo apachectl configtest && sudo systemctl reload apache2
```

On a fresh install, the distribution's default site also answers on port 80. That doesn't matter as long as `server_name`/`ServerName` match your host name, but you can disable it (`sudo rm /etc/nginx/sites-enabled/default`, or `sudo a2dissite 000-default`).

**5. The certificate.** The deploy hook reloads the proxy whenever certbot gets a new certificate, so renewals take effect without you doing anything. Use `systemctl reload apache2` for Apache.

```bash
sudo certbot certonly --webroot -w /var/www/letsencrypt \
  -d angel.example.com \
  --deploy-hook "systemctl reload nginx"
```

certbot asks for an e-mail address for expiry warnings and for agreement to the terms. The certificate lands in `/etc/letsencrypt/live/angel.example.com/`, which is where `angel.conf` expects it.

**6. HTTPS.** Install `angel.conf` and `angel-proxy.conf`, and replace the `CHANGE` values in `angel.conf`.

```bash
# nginx
sudo mkdir -p /etc/nginx/snippets
sudo cp nginx/angel-proxy.conf /etc/nginx/snippets/angel-proxy.conf
sudo cp nginx/angel.conf /etc/nginx/conf.d/angel.conf
sudo nginx -t && sudo systemctl reload nginx

# Apache
sudo cp apache/angel-proxy.conf /etc/apache2/angel-proxy.conf
sudo cp apache/angel.conf /etc/apache2/sites-available/angel.conf
sudo a2enmod proxy proxy_http headers ssl
sudo a2ensite angel
sudo apachectl configtest && sudo systemctl reload apache2
```

**7. Renewals.** Let's Encrypt certificates last 90 days. The `certbot` package installs a systemd timer that renews them when 30 days are left (`systemctl list-timers | grep certbot` shows it). Test the whole renewal, including the challenge over port 80, with:

```bash
sudo certbot renew --dry-run
```

**8. Check it.** Open `https://angel.example.com` on a phone using mobile data, then run [`check.sh`](#checking-a-setup) against it.

The configs send `Strict-Transport-Security` for a year. Once a browser has seen it, that browser refuses plain HTTP for the host name, so get HTTPS working before you rely on it.

## Trying it locally

Each demo builds Angel from this repository and puts the proxy in front of it on `http://localhost:8080` (set `HOST_PORT` for another port). Angel isn't published, so the only way in is through the proxy.

```bash
cd nginx     # or apache
docker compose up --build
```

Open `http://localhost:8080` and enter `1234` (Caregiver) or `987654` (Client). These PINs are for the demo only. The demo uses plain HTTP; browsers accept the `Secure` cookie anyway, because it's `localhost`.

The two containers have fixed addresses on their own network, and `TRUSTED_PROXIES` is the proxy's address (`172.30.0.10`). If that subnet is already in use on your machine, change it in `compose.yml`.

## Checking a setup

```bash
./check.sh http://localhost:8080                       # the demo
CAREGIVER_PIN=<your PIN> ./check.sh https://angel.example.com  # your server
```

The script checks that, through the proxy:

1. `GET /healthz` answers 200,
2. the Caregiver PIN logs in and sets the session cookie,
3. five wrong PINs are refused and the sixth is rate limited (429),
4. a wrong PIN sent with a made-up `X-Forwarded-For` is still rate limited. If the proxy passed that header on unchanged, this attempt would count as a new address and get through.

Running it uses up the rate limit for your address for 15 minutes. The limit is kept in memory, so restarting Angel resets it (`docker compose restart angel` in the demo).

The script can't check that different visitors get different addresses, because it runs from just one. Check `TRUSTED_PROXIES` against [the settings above](#settings-for-angel) for that.

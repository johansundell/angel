# Deploying releases to a VPS automatically

This guide sets up a VPS so that each new GitHub release of angel is installed on it automatically. The setup has three parts:

- **On the server:** [`scripts/deploy.sh`](../scripts/deploy.sh) downloads the release for a tag, checks its SHA-512 checksum, replaces the binary and restarts the service. If `/healthz` does not answer within about 30 seconds, it puts the previous binary back.
- **On GitHub:** the `Deploy` workflow ([`.github/workflows/deploy.yml`](../.github/workflows/deploy.yml)) runs when a release is published. It connects to the server over SSH and sends the tag. That is all it does.
- **Between them:** a `deploy` user whose SSH key can run `deploy.sh` and nothing else. GitHub never gets a shell or file access on the server.

nginx needs no changes, because angel keeps listening on the same port.

The guide assumes this setup:

- A Linux VPS, with angel installed in `/opt/angel` as the systemd service `angel` (`./angel -service install`).
- nginx in front of angel, which listens on `127.0.0.1:8080`.
- Releases made with `make release`, which uploads `angel_<tag>_linux_<arch>.tar.gz` and a `.sha512` file to the GitHub release.

Do the steps in order. Each one ends with a check.

## 1. Check the current install

On the VPS:

```bash
systemctl cat angel
```

Look at `ExecStart`. It must be `/opt/angel/angel` (sometimes with arguments). Then check the port in the service's `.env`:

```bash
grep '^PORT' /opt/angel/.env
curl -s -H 'Accept: application/json' http://127.0.0.1:8080/healthz
```

The `curl` command should print JSON with `"dbStatus":"OK"` and the running `version`.

If the binary path, the service name or the port is different, edit `DIR`, `SERVICE` or `HEALTH_URL` at the top of `deploy.sh` in step 2.

## 2. Install the deploy script

Copy the script to the server and make it root-owned. The `deploy` user must not be able to change it, because it runs as root:

```bash
# on your machine
scp scripts/deploy.sh you@your.vps:/tmp/deploy.sh

# on the VPS
sudo install -o root -g root -m 0755 /tmp/deploy.sh /opt/angel/deploy.sh
rm /tmp/deploy.sh
```

The script needs `curl`, `tar` and `sha512sum`. These are installed on Debian and Ubuntu by default.

**Check:** reinstall the version that is already running. Use the `version` from step 1, for example:

```bash
sudo /opt/angel/deploy.sh v0.0.11
```

It should end with the `/healthz` JSON and `deployed v0.0.11`. The previous binary is now in `/opt/angel/angel.prev`.

> The server runs its own copy of the script. If `scripts/deploy.sh` changes in the repo, repeat this step.

## 3. Create the deploy user

On the VPS:

```bash
sudo useradd --system --create-home --shell /bin/bash deploy
echo 'deploy ALL=(root) NOPASSWD: /opt/angel/deploy.sh' | sudo tee /etc/sudoers.d/angel-deploy
sudo chmod 0440 /etc/sudoers.d/angel-deploy
sudo visudo -c
```

The user needs a real shell, because SSH runs the forced command in step 4 through it. The sudoers rule lets `deploy` run this one script as root, without a password, and nothing else.

**Check:** `visudo -c` reports `parsed OK`.

## 4. Create the SSH key

On your own machine, create a key that is used only for deploys. Don't add a passphrase, because GitHub has to use it without anyone typing one:

```bash
ssh-keygen -t ed25519 -N '' -C 'angel deploy (GitHub Actions)' -f angel_deploy_key
```

This creates `angel_deploy_key` (private, goes to GitHub in step 6) and `angel_deploy_key.pub` (public, goes on the server).

On the VPS, allow the public key, but only for running the deploy script:

```bash
sudo install -d -o deploy -g deploy -m 0700 /home/deploy/.ssh
sudo tee /home/deploy/.ssh/authorized_keys >/dev/null <<'EOF'
command="sudo /opt/angel/deploy.sh \"$SSH_ORIGINAL_COMMAND\"",restrict PASTE-THE-CONTENTS-OF-angel_deploy_key.pub-HERE
EOF
sudo chown deploy:deploy /home/deploy/.ssh/authorized_keys
sudo chmod 0600 /home/deploy/.ssh/authorized_keys
```

The result is one line: `command="…",restrict ssh-ed25519 AAAA… angel deploy (GitHub Actions)`.

- `command=` means that whatever the client asks to run, the server runs `deploy.sh` with it as the tag. `deploy.sh` rejects anything that is not `vX.Y.Z`.
- `restrict` turns off port forwarding, agent forwarding and the terminal.

**Check:** from your machine, deploy the running version through the key:

```bash
ssh -i angel_deploy_key deploy@your.vps v0.0.11
```

This should print the same output as in step 2. Then try something that is not a tag:

```bash
ssh -i angel_deploy_key deploy@your.vps 'ls /'
```

This should print `usage: deploy.sh vX.Y.Z (got: 'ls /')` and do nothing else.

## 5. Get the server's host key

GitHub needs the server's host key, so that it can check it is connecting to your server. On your machine:

```bash
ssh-keyscan -t ed25519 your.vps > angel_known_hosts
ssh-keygen -lf angel_known_hosts
```

On the VPS, compare the fingerprint with the server's own:

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

**Check:** both fingerprints are the same. If SSH listens on a port other than 22, add `-p <port>` to `ssh-keyscan` and see the troubleshooting section.

## 6. Add the secrets on GitHub

In the repository, go to **Settings → Environments → New environment** and create `production`. Optionally, under **Deployment branches and tags**, allow only tags that match `v*`.

Add three **environment secrets**:

| Secret | Value |
|---|---|
| `DEPLOY_HOST` | The VPS hostname or IP, for example `your.vps` |
| `DEPLOY_SSH_KEY` | The full contents of `angel_deploy_key`, including the `BEGIN` and `END` lines |
| `DEPLOY_KNOWN_HOSTS` | The contents of `angel_known_hosts` |

Then delete the private key from your machine, or move it somewhere safe:

```bash
rm angel_deploy_key
```

## 7. Test the workflow by hand

Go to **Actions → Deploy → Run workflow**, enter the running version (for example `v0.0.11`) and run it.

**Check:** the job is green, and its log ends with `deployed v0.0.11`.

## 8. Release as usual

From now on, `make release` also deploys:

1. `make release` creates the GitHub release with your token. Publishing the release starts the `Deploy` workflow.
2. `deploy.sh` waits for the release files to finish uploading. It retries for up to about five minutes.
3. The binary is replaced, the service restarts, and `/healthz` is checked.

**Pre-releases** (releases marked "This is a pre-release" on GitHub) are not deployed.

**To go back to an older version**, run the workflow by hand (step 7) with the older tag.

## Troubleshooting

- **The workflow did not start after a release.** Releases created by a workflow with the built-in `GITHUB_TOKEN` don't start other workflows. `make release` uses your own token, so this only happens if releases are created another way. Run the workflow by hand.
- **`Host key verification failed`.** `DEPLOY_KNOWN_HOSTS` doesn't match the server. Repeat step 5. This also happens after reinstalling the VPS.
- **`Permission denied (publickey)`.** Check that the public key line in `/home/deploy/.ssh/authorized_keys` is one line, and that the file is owned by `deploy` with mode `0600`. Run `sudo journalctl -u ssh -n 50` on the server for the reason.
- **`sudo: a password is required`.** The sudoers path must match exactly: `/opt/angel/deploy.sh`.
- **SSH on a port other than 22.** Change the `ssh` line in the workflow to `ssh -p <port> -o BatchMode=yes deploy@"$HOST" "$TAG"`, and make sure `angel_known_hosts` was made with `ssh-keyscan -p <port>`.
- **`rolling back`.** The new version started but `/healthz` didn't answer with 200. The previous binary is running again. Run `journalctl -u angel -n 100` on the server for the reason. Then fix the problem and release again, or run the workflow by hand with the same tag.
- **`curl: (22) … 404` after many retries.** The release has no file for this server's architecture. Check that `make release` built `linux_amd64` (or `linux_arm64` on an ARM VPS).

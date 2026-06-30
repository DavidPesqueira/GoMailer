# GoMailer

CLI email tool inspired by Kali Linux's `sendEmail`.

![GoMailer](/GoMailer.webp)

A simple Go tool for sending emails with custom headers, attachments, and inline images — useful for authorized phishing simulations and red team engagements. Includes a built-in domain authentication posture checker (`--check`) to assess SPF/DKIM/DMARC configuration before an engagement.

## Prerequisites

- Go 1.18 or higher
- SMTP server credentials (see note on app passwords below)

## Installation

Clone the repository and install dependencies:

```bash
git clone https://github.com/DavidPesqueira/GoMailer.git
cd GoMailer
sh setup.sh
go mod tidy
```

If you pull updates and hit a build error like:
checkdomain.go:2:1: expected 'package', found 'import'

it means `checkdomain.go` is missing its package declaration at the top (can happen after manual edits via the GitHub web editor). Fix with:

```bash
sed -i '' '1i\
package main\
' checkdomain.go
```

(The empty `''` after `-i` is required on macOS's BSD `sed` — omit it on Linux/GNU `sed`.) Or just open the file and add `package main` as the first line by hand.

## Configuration

Create a `config.ini` file in the project root with your SMTP credentials:

```ini
[SMTP]
server=smtp.example.com
port=587
username=your-username
password=your-app-password
```

`config.ini` is gitignore — never commit real credentials. Use `config.ini.example` as a template for what to fill in.

> **Note on credentials:** Most providers will **not** accept your normal account password here. You'll need a provider-issued **app password** or **SMTP key**:
> - **Brevo** — server is `smtp-relay.brevo.com`, port `587`. Username is your Brevo login email (or the assigned `xxxxxxx@smtp-brevo.com` address), password is the **SMTP key** generated under *SMTP & API → SMTP* in the dashboard.
> - **Gmail** — generate an [App Password](https://myaccount.google.com/apppasswords) (requires 2FA). Username is your full address.
> - **Outlook / Office 365** — app password if MFA is enabled.
> - **SendGrid** — username is literally `apikey`, password is your API key.
> - **Mailgun / SES / Postmark** — use the dedicated SMTP credentials they issue, not your login.
>
> Port `587` uses STARTTLS; port `465` uses implicit TLS/SSL. Match the port to your provider's expected mode.

### Example: Brevo config

```ini
[SMTP]
server=smtp-relay.brevo.com
port=587
username=you@example.com
password=your-brevo-smtp-key
```

## Usage

### Sending email

Build and run:

```bash
go run .
```

or build a standalone binary:

```bash
go build -o gomailer
./gomailer
```

Follow the prompts to enter sender details, recipient email, subject, body, and optional attachments or inline images.

### Domain authentication posture check (`--check`)

Before any engagement involving a sender domain — your own or a target's — check its SPF/DKIM/DMARC posture:

```bash
./gomailer --check example.com
```

This reports:
- **MX records** for the domain
- **SPF record**, if published
- **DKIM** — best-effort probe against common selector names (`default`, `selector1`, `google`, etc.); absence here doesn't rule out DKIM, since the real selector is usually only visible in a captured message's `DKIM-Signature` header
- **DMARC record and policy** (`p=none` / `quarantine` / `reject`), with a plain-English verdict on whether exact-domain spoofing is likely to land in the inbox

Use this to:
- Confirm your own sending domain is properly authenticated before a campaign (run it against your own domain post-setup)
- Document a target domain's authentication posture for an engagement report
- Decide whether exact-domain sending is viable or whether a cousin-domain / display-name approach is needed, based on the receiving domain's actual published policy

## Troubleshooting

**`535 5.7.8 Authentication failed`** — Wrong credential type. Use a provider-issued app password / SMTP key, not your account login password.

**`530 5.7.0 Authentication required`** — Server expects auth but none was sent. Check that `username` and `password` are populated and the `[SMTP]` section header matches what the code reads.

**`534 5.7.9 Application-specific password required`** — Provider (usually Gmail) is telling you directly to switch to an app password.

**Connection hangs or TLS errors** — Port/encryption mismatch. `587` = STARTTLS, `465` = implicit TLS. Match the port to your provider's expected mode.

**Mail silently not arriving (not even to spam)** — Usually a hard authentication failure (SPF/DKIM/DMARC misalignment) rather than a content/spam-scoring issue. Run `./gomailer --check <your-sending-domain>` to confirm your records are correct and aligned with your sending provider. Outlook/Microsoft and most major providers now reject unauthenticated mail outright rather than spam-foldering it.

**`expected 'package', found 'import'` on build** — See the Installation section above; `checkdomain.go` is missing its `package main` line.

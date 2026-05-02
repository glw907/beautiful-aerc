// SPDX-License-Identifier: MIT

package config

// Template returns poplar's self-documenting config.toml template.
// poplar writes this to disk on first launch when no config exists.
//
// The output is intentionally checked against a golden file so any
// formatting drift surfaces in code review.
func Template() string {
	return templateBody
}

const templateBody = `# poplar — terminal email client
# https://github.com/glw907/poplar
#
# This file configures poplar's mail accounts and UI. poplar
# reads it at startup, so restart poplar after editing.
#
# Location:
#   Linux (Ubuntu/Mint)   ~/.config/poplar/config.toml
#   macOS                 ~/.config/poplar/config.toml
#   Windows               %APPDATA%\poplar\config.toml
#
# (poplar follows the Linux XDG convention on macOS too; Apple's
# ~/Library/Application Support/ is reserved for GUI apps.)
#
# The file uses TOML syntax. The two top-level structures are:
#
#   [[account]]   one block per mail account; multiple allowed
#   [ui]          one block of UI preferences for all accounts
#
# Every option below ships with a sensible default. To override
# a default, uncomment its line and change the value.


# ──────────────────────────────────────────────────────────────────────
#  ACCOUNTS
# ──────────────────────────────────────────────────────────────────────

# Provider presets
# ────────────────
#
# A "preset" is a built-in shortcut that fills in the right
# server name, port, and auth method for a known mail host. Set
` + "`provider`" + ` to a preset name and poplar handles the rest — you
# only supply your email and password.
#
# If your provider isn't listed (or you run your own server),
# use one of the fallbacks at the bottom and supply the
# connection details by hand.
#
#   fastmail      Fastmail (JMAP)
#   gmail         Google Mail (IMAP, OAuth)
#   icloud        Apple iCloud Mail (IMAP, app password)
#   yahoo         Yahoo Mail (IMAP, app password)
#   zoho          Zoho Mail (IMAP, app password)
#   outlook       Microsoft Outlook / Office 365 (IMAP, OAuth)
#   mailbox-org   Mailbox.org (IMAP, app password)
#   posteo        Posteo (IMAP)
#   runbox        Runbox (IMAP)
#   gmx           GMX (IMAP)
#   protonmail    ProtonMail via local Bridge — see notes below
#
# Fallbacks for unlisted or self-hosted servers:
#
#   imap          any IMAP server (set host/port below)
#   jmap          any JMAP server (set source URL below)


# Setup notes
# ───────────
#
# Most presets work with email + password. A few require extra
# setup; this section covers the cases that aren't obvious from
# the preset name.
#
# App-password providers
#
#   icloud, yahoo, zoho, mailbox-org, posteo, runbox, gmx, and
#   fastmail (when used over IMAP) all require an app-specific
#   password generated in your account settings — your normal
#   login password won't work over IMAP. Each preset's HelpURL
#   in ` + "`poplar config check`" + ` output points to the correct page.
#
# OAuth providers
#
#   gmail and outlook authenticate via OAuth, not a password.
#   poplar runs the OAuth flow on first connect and caches the
#   refresh token via your ` + "`password-cmd`" + `. See:
#
#       https://github.com/glw907/poplar/blob/master/docs/oauth.md
#
# ProtonMail
#
#   ProtonMail isn't reachable over IMAP directly. Install the
#   Bridge app on this machine; it exposes Proton's encrypted
#   mailbox as a local IMAP server.
#
#       Linux      protonmail-bridge from proton.me
#                  (Snap, .deb, or AppImage)
#       macOS      Proton Mail Bridge.app from proton.me
#       Windows    installer from proton.me/mail/bridge
#
#   A paid Proton plan is required. Bridge generates a
#   per-account password (different from your Proton login)
#   to use as ` + "`password`" + ` in poplar. The protonmail preset
#   assumes Bridge's default port (1143); override with
#   ` + "`port = ...`" + ` if you've changed Bridge's listen port.


# Secrets
# ───────
#
# poplar never stores cleartext passwords in this file. Instead
# you tell poplar where to fetch the secret at startup, using
# one of two patterns.
#
# 1. Environment variable
#
#       password = "$VAR_NAME"
#
#    poplar reads $VAR_NAME from its own environment at startup.
#    Easy to set up if you already source secrets from your
#    shell config — but the variable lives in every process
#    that inherits the environment, which isn't ideal for
#    sensitive credentials.
#
# 2. Command (recommended)
#
#       password-cmd = "<shell command>"
#
#    poplar runs the command on each reconnect; standard output
#    becomes the secret. This pairs naturally with secret
#    managers that expose a CLI, and the password never sits in
#    your environment or shell history.
#
#    Examples by OS:
#
#    Linux (Ubuntu, Mint, Debian)
#         op read op://Personal/Fastmail/credential   (1Password)
#         pass show mail/fastmail                     (pass)
#         secret-tool lookup service poplar account   (libsecret)
#
#    macOS
#         op read op://Personal/Fastmail/credential   (1Password)
#         security find-generic-password -w -s poplar -a fastmail
#
#    Windows (PowerShell SecretManagement)
#         pwsh -Command "Get-Secret -Name fastmail -AsPlainText"
#         op read op://Personal/Fastmail/credential   (1Password)
#
# Pick whichever fits your setup. If you don't have a secret
# manager yet:
#
#   Linux: install ` + "`pass`" + ` — it stores secrets as GPG-encrypted
#          files under ~/.password-store with zero dependencies
#          beyond GPG.
#   macOS: the built-in ` + "`security`" + ` tool above already works;
#          add an entry with ` + "`security add-generic-password`" + `.
#   Windows: install Microsoft.PowerShell.SecretManagement and
#          Microsoft.PowerShell.SecretStore (PowerShell Gallery),
#          then ` + "`Set-Secret -Name fastmail`" + `.


# Hosted-provider example
# ───────────────────────

[[account]]
provider     = "fastmail"
email        = "you@yourdomain.com"
password-cmd = "op read op://Personal/Fastmail/credential"


# Optional account fields (defaults shown)
# ────────────────────────────────────────
#
#   name = "<email>"
#       Display name in the sidebar account header.
#
#   port = 993
#       IMAP fallback only. Preset providers fill this in.
#
#   starttls = false
#       IMAP fallback only. The default (false) uses implicit
#       TLS — encryption is established before any IMAP traffic.
#       Set true if your server expects plaintext IMAP first
#       and then upgrades to TLS via STARTTLS.
#
#   insecure-tls = false
#       Skip TLS certificate verification. Required only for
#       self-signed certs (homelab IMAP). The protonmail preset
#       enables this automatically. Leave false for hosted
#       providers.
#
#   auth = "plain"
#       SASL mechanism for IMAP. plain works with every server
#       you'll plausibly hit; cram-md5 is a slightly older
#       challenge-response variant; xoauth2 is for OAuth-based
#       providers (gmail/outlook presets set this for you).
#       JMAP doesn't use SASL; it always uses bearer tokens.
#
#   copy-to = "Sent"
#       Folder where sent-message copies are saved.
#
#   folders-sort = ["Inbox", "Sent", "Archive"]
#       Folder order in the sidebar within each group. Folders
#       you don't list appear after listed ones in the order
#       the server returns them.


# Self-hosted IMAP example
# ────────────────────────
# [[account]]
# provider = "imap"
# host     = "mail.example.com"
# email    = "user@example.com"
# password = "$IMAP_PASSWORD"


# ProtonMail via Bridge example
# ─────────────────────────────
# [[account]]
# provider     = "protonmail"
# email        = "you@protonmail.com"
# password-cmd = "op read op://Personal/ProtonBridge/password"
# port         = 1143  # override only if Bridge is on a non-default port


# ──────────────────────────────────────────────────────────────────────
#  UI
# ──────────────────────────────────────────────────────────────────────
#
# A single [ui] block applies to all accounts.

[ui]
# theme = "one-dark"
#     Compiled theme name. See docs/poplar/styling.md for the
#     full list (gruvbox, nord, dracula, monokai, …).
#
# undo_seconds = 6
#     Triage toast/undo window. Range 2-30.
#
# trash_retention_days = 0
#     Auto-empty messages older than N days from Trash on first
#     load each session. 0 disables. Range 0-365.
#
# spam_retention_days = 0
#     Same as above, for Spam.
`

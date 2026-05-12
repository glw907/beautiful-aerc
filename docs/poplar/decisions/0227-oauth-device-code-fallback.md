---
title: OAuth device-code fallback
status: accepted
date: 2026-05-12
---

## Context

Audit E F6 (ADR-0225): ADR-0193 picked loopback PKCE as the only
consent flow and explicitly rejected device-code, citing Google
CASA display-name verification. The CASA rationale was misread —
CASA gates *maintainer-distributed* clients, not user-driven
device-code grants against the user's own BYO client. The
loopback-only choice forecloses SSH-tunnel users, container
users, and NAT-bound users who can't open a local port that the
browser can reach.

## Decision

Add RFC 8628 device-code as an opt-in fallback to ADR-0193's
loopback PKCE default. Loopback stays the recommended path.

- `mailauth.Config.DeviceAuthURL` carries the provider's device-
  auth endpoint; `OAuthDefaults.DeviceAuthURL` propagates it from
  the preset table. Gmail and Outlook gain the device endpoints
  (`https://oauth2.googleapis.com/device/code` /
  `https://login.microsoftonline.com/common/oauth2/v2.0/devicecode`).
- `mailauth.Client` exposes the device-code flow as two methods:
  `RequestDeviceAuth(ctx) (*DeviceAuth, error)` POSTs the
  device-auth request and returns the parsed response (user_code,
  verification_uri, device_code, expires_in, interval);
  `PollDeviceCode(ctx, *DeviceAuth) error` polls the token
  endpoint per `interval` until success / `access_denied` /
  `expired_token`, with RFC 8628 §3.5 `slow_down` adding 5s.
  `AuthorizeDeviceCode(ctx, DeviceDisplay)` is the convenience
  wrapper for callers that don't need intermediate state.
- Wizard surface: the existing `oauthStageCredentials` form gains
  a `huh.NewSelect` radio "Consent method: Loopback (recommended)
  / Device code" when the preset supplies `DeviceAuthURL`. After
  a loopback failure the retry screen adds a `[d] switch to
  device code` affordance alongside `[r] retry` / `[s] cancel`.
- Wizard splits the device flow into two Cmds so View can render
  the user_code while the second Cmd polls: `runDeviceAuthorize`
  hits `RequestDeviceAuth` and emits `oauthDeviceCodeMsg`;
  `updateFlow` stashes the user_code + URI and kicks off
  `runDevicePoll`, which returns `oauthDoneMsg` when the token
  lands. The new `oauthStageDevice` stage owns the display.
- New `[account] oauth-mode = "loopback"` (default, omitted on
  round-trip) or `"device-code"`. Set by the wizard so
  `--reauth` can preserve the user's chosen method. Decoded via
  `validateOAuthMode` (ConfigError on unknown values, sibling of
  `validateOAuthStore`).

## Consequences

Unblocks SSH/container/NAT users who can't host a loopback port.
The token-store + refresh path is unchanged — once the device
flow stores a refresh token, `(*Client).Token` is mode-agnostic.

BYO-client setup needs the user to register the right OAuth
client type at the provider — Google requires "TVs and Limited
Input devices," Microsoft requires a public client with device-
code enabled. The wizard's help URL still points at the
provider's app registration page; the rest is user-side.

Supersedes the "Forecloses device-code in v1" clause from
ADR-0193; the rest of ADR-0193 (BYO client, loopback PKCE as
default, keyring/age-file store, `--reauth` CLI surface) stands.

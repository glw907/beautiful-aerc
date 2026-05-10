# OAuth setup for Gmail and Outlook

Gmail and Outlook require XOAUTH2 for IMAP and SMTP. Poplar cannot
ship a pre-verified OAuth client: Google's CASA audit and Microsoft's
publisher verification process are both out of scope for an open-source
terminal client. Instead, you register your own application in the
provider's developer console — one ten-minute process per provider —
paste the credentials into the wizard, and poplar manages the token
lifecycle from there. You own the client; the consent screen names your
own account.

## Gmail

1. Open [Google Cloud Console](https://console.cloud.google.com/) and
   sign in with the account you want to configure.

2. Click the project selector at the top, then **New Project**. Name it
   anything (e.g. "Poplar mail"). Click **Create**.

3. In the left nav, open **APIs & Services > Library**. Search for
   **Gmail API** and enable it.

4. Go to **APIs & Services > OAuth consent screen**.
   - User type: **External**.
   - Fill in the app name and your email as developer contact.
   - On the **Scopes** page, click **Add or Remove Scopes** and add
     `https://mail.google.com/` (the full-access Gmail scope).
   - On the **Test users** page, add your Gmail address. Testing status
     is fine for personal use — Google does not time-limit test-mode
     tokens for your own account.
   - Save and continue.

5. Go to **APIs & Services > Credentials > Create Credentials >
   OAuth client ID**.
   - Application type: **Desktop app**.
   - Click **Create**.

6. Copy the **Client ID** and **Client Secret** from the dialog.

7. Run `poplar config init --interactive` (first account) or
   `poplar --repair=<name>` (existing account). When the wizard reaches
   the credential screen for the `gmail` preset, paste the client ID
   and client secret into the respective fields.

8. The wizard runs a loopback consent flow. A browser window opens for
   Google sign-in. Approve access, and poplar saves the refresh token.

## Outlook

1. Open [Azure Portal](https://portal.azure.com/) and sign in.

2. Go to **Azure Active Directory > App registrations > New
   registration**.
   - Name: anything (e.g. "Poplar mail").
   - Supported account types: **Personal Microsoft accounts only**
     (or "Accounts in any organizational directory and personal
     accounts" if you also use a work tenant).
   - Click **Register**.

3. In the app's left nav, go to **Authentication > Add a platform >
   Mobile and desktop applications**.
   - Tick the checkbox for `https://login.microsoftonline.com/common/oauth2/nativeclient`.
   - Under **Allow public client flows**, set to **Yes**.
   - Click **Save**.

4. Go to **API permissions > Add a permission > Microsoft Graph >
   Delegated permissions**. Add:
   - `IMAP.AccessAsUser.All`
   - `SMTP.Send`
   - `offline_access`

   Click **Add permissions**. You do not need admin consent for
   delegated personal-account permissions; the first sign-in grants
   them.

5. Copy the **Application (client) ID** from the Overview page.

6. Run `poplar config init --interactive` or `poplar --repair=<name>`.
   Paste the Application ID as the client ID. For the client secret
   field, paste a placeholder (`""`). Outlook public-client apps
   authenticate via the device authorization flow or PKCE without a
   secret. Poplar v1 sends the field but accepts blank for Outlook's
   public-client endpoints; the wizard will not reject it. Full
   public-client support without a secret placeholder is a known
   follow-up item.

7. The wizard opens a browser window for Microsoft sign-in. Approve the
   requested permissions. Poplar saves the refresh token.

## When the refresh token expires

Google rotates refresh tokens after approximately six months of
inactivity, or when you revoke access in Google Account settings.
Microsoft refresh tokens expire after 90 days of non-use for personal
accounts.

To renew: run `poplar --reauth=<name>` where `<name>` matches the
account name in your config. The wizard re-runs the consent flow for
that account and overwrites the stored refresh token. No other account
data changes.

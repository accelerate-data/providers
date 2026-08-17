# Authentication Providers

This file describes the requirements that a tool must meet in order to function properly as an Obot auth provider.

## Requirements

### Metadata

The tool must have a metadata line in the `tool.gpt` called `envVars`, that lists all the required configuration
parameters for the auth provider.

Optionally, the tool may also include an `optionalEnvVars` metadata line, that lists all the optional configuration parameters.

Example from the GitHub auth provider:

```
...
Metadata: envVars: OBOT_GITHUB_AUTH_PROVIDER_CLIENT_ID,OBOT_GITHUB_AUTH_PROVIDER_CLIENT_SECRET,OBOT_AUTH_PROVIDER_COOKIE_SECRET,OBOT_AUTH_PROVIDER_EMAIL_DOMAINS
Metadata: optionalEnvVars: OBOT_GITHUB_AUTH_PROVIDER_TEAMS,OBOT_GITHUB_AUTH_PROVIDER_ORG,OBOT_GITHUB_AUTH_PROVIDER_REPO,OBOT_GITHUB_AUTH_PROVIDER_TOKEN,OBOT_GITHUB_AUTH_PROVIDER_ALLOW_USERS
...
```

### Implementation Details

The auth provider must implement user authentication using OAuth2. The user should be able to log in using a standard
OAuth2 authorization code flow.

#### Token Cookie

The auth provider must store the token in a cookie called `obot_access_token`.
This cookie should be set as `Secure` only if the `OBOT_SERVER_PUBLIC_URL` environment variable starts with `https://`.
The cookie should be encrypted using the `OBOT_AUTH_PROVIDER_COOKIE_SECRET` environment variable.

#### URL Paths

The auth provider must implement the following URL paths:

- `/oauth2/start`: This path should start the OAuth2 flow by sending the user to the OAuth2 provider's authorization URL.
  - It must check for the `rd` query parameter. This is the URL to redirect the user to after the full OAuth2 flow is complete.
    This value can be stored alongside the state if needed.
- `/oauth2/callback`: This path should handle the OAuth2 callback from the OAuth2 provider.
  - After exchanging the code for the access token, it should redirect the user to the URL stored in the `rd` query parameter
    from the `/oauth2/start` request.
- `/oauth2/sign_out`: This path should sign the user out by clearing the `obot_access_token` cookie and redirecting the user to
  the URL in the `rd` query parameter.
- `/obot-get-icon-url`: This path should take the user's access token from the `Authorization` header and use it to get
  the user's profile picture URL. It should return a JSON object with the URL in the `iconURL` field.
  The `Authorization` header will be in the format `Bearer <access token>`.
- `/obot-get-state`: More details in the next section.

##### Obot-Get-State

`/obot-get-state` is the path that Obot uses to get information about the user making a request.
Requests to this path will include a JSON body with the following JSONSchema:

```json
{
  "type": "object",
  "properties": {
    "method": {
      "type": "string",
      "description": "The HTTP method of the request (e.g., GET, POST)."
    },
    "url": {
      "type": "string",
      "format": "uri",
      "description": "The URL of the request."
    },
    "header": {
      "type": "object",
      "additionalProperties": {
        "type": "array",
        "items": {
          "type": "string"
        }
      },
      "description": "Headers of the request, where keys are header names and values are arrays of header values."
    }
  },
  "required": ["method", "url", "header"],
  "additionalProperties": false
}
```

This object represents a serialized HTTP request that Obot received from a user.
The auth provider must return information about the authenticated user that made this request.
Under most (if not all) circumstances, the auth provider only needs to look at the cookie header,
which it can then decrypt and use to get information about the user.

The auth provider must return a JSON object with information about the user,
matching the following JSONSchema:

```json
{
  "type": "object",
  "properties": {
    "accessToken": {
      "type": "string",
      "description": "The access token for the user."
    },
    "preferredUsername": {
      "type": "string",
      "description": "The preferred username of the user."
    },
    "user": {
      "type": "string",
      "description": "The identifier for the user."
    },
    "email": {
      "type": "string",
      "format": "email",
      "description": "The email address of the user."
    },
    "issuer": {
      "type": "string",
      "description": "The OIDC issuer that minted the user identifier. Required for generic OIDC providers."
    },
    "emailVerified": {
      "type": "boolean",
      "description": "Whether the issuer reports the email address as verified. Generic OIDC providers should include this when the upstream issuer returns the claim."
    }
  },
  "required": ["accessToken", "preferredUsername", "user", "email"],
  "additionalProperties": true
}
```

Here is an example:

```json
{
  "accessToken": "xyz",
  "preferredUsername": "johndoe",
  "user": "johndoe",
  "email": "johndoe@example.com"
}
```

If the `obot_access_token` cookie is not present or is invalid, the auth provider should return a 400 status code.

### Generic OAuth / OIDC Provider

The generic provider is implemented by `generic-oauth-auth-provider` and registered by
`auth-providers/generic-oauth-auth-provider.yaml`.

Required configuration:

- `OBOT_GENERIC_OAUTH_AUTH_PROVIDER_NAME`: Display name shown on the login page.
- `OBOT_GENERIC_OAUTH_AUTH_PROVIDER_ISSUER`: OIDC issuer URL. The issuer must support discovery at `/.well-known/openid-configuration`.
- `OBOT_GENERIC_OAUTH_AUTH_PROVIDER_CLIENT_ID`: OAuth/OIDC client ID.
- `OBOT_GENERIC_OAUTH_AUTH_PROVIDER_CLIENT_SECRET`: OAuth/OIDC client secret.
- `OBOT_AUTH_PROVIDER_COOKIE_SECRET`: Base64-encoded cookie secret.
- `OBOT_AUTH_PROVIDER_EMAIL_DOMAINS`: Allowed email domains, or `*`.
- `OBOT_GENERIC_OAUTH_AUTH_PROVIDER_TRUST_EMAIL_LINKING`: Whether Obot may link accounts by verified email for this issuer.

Optional configuration:

- `OBOT_GENERIC_OAUTH_AUTH_PROVIDER_SCOPE`: OAuth scopes. Defaults to `openid email profile`.
- `OBOT_AUTH_PROVIDER_POSTGRES_CONNECTION_DSN`: PostgreSQL session storage DSN.
- `OBOT_AUTH_PROVIDER_TOKEN_REFRESH_DURATION`: Token refresh duration. Defaults to `1h`.
- `OBOT_AUTH_PROVIDER_ENABLE_LOGGING`: Enables oauth2-proxy request, auth, and standard logging.

The generic provider uses OIDC discovery, ID token validation, issuer verification, audience verification, nonce validation,
and PKCE `S256` through OAuth2 Proxy. Its `/obot-get-state` response sets `user` to the OIDC `sub`, sets `issuer` to the
configured issuer, and forwards `emailVerified` from the upstream `email_verified` claim when present.

## Reference Implementation

The Google auth provider in this repo should be considered the standard reference implementation.
It follows all the requirements listed above and can be used as a reference when implementing a new auth provider.
It is recommended to make use of the [OAuth2 Proxy](https://github.com/obot-platform/oauth2-proxy) like the Google and GitHub auth providers do.

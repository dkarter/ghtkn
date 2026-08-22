---
description: Revoke leaked or compromised ghtkn access tokens. Use during incident response, when a token leaks, or to revoke an app stored token via ghtkn revoke or the GitHub REST API.
---

# How To Revoke Access Tokens

If an access token is leaked, it must be immediately invalidated.
[You can confirm if the leaked access token expires or not by GitHub API.](https://docs.github.com/en/rest/users/users?apiVersion=2022-11-28#get-the-authenticated-user)

## `ghtkn revoke`

`version >= v0.2.7`

The simplest way is the `ghtkn revoke` command:

```sh
ghtkn revoke <app name>        # revoke the token stored for an app and delete it from the backend
ghtkn revoke                   # revoke the token stored for GHTKN_APP or the default app
ghtkn revoke --all             # revoke the stored tokens of every app in the config
```

Do not put a raw token on the command line. Command arguments can be recorded in shell history and may be visible to other processes. Read it without echo and pass it through standard input instead:

```sh
read -rsp "Token to revoke: " token_to_revoke
printf '\n'
printf '%s\n' "$token_to_revoke" | ghtkn revoke --token-stdin
unset token_to_revoke
```

`--token-stdin` reads one raw token from the first line. It can be combined with app names or `--all` when both the supplied token and stored tokens must be revoked. Positional raw tokens remain supported for compatibility, but they are unsafe and should not be used.

Positional arguments are treated as app names unless they have a recognized GitHub token prefix (`ghp_`, `github_pat_`, `gho_`, `ghu_`, `ghr_`). When no argument or standard-input token is given, the command falls back to `GHTKN_APP` or the default app. When only a raw token is supplied, the fallback is not used, so revoking it never touches an unrelated app's stored token.

The `--all` flag revokes the stored tokens of every app in the config at once. This is meant for incident response: when the environment running ghtkn is compromised, you can revoke all stored tokens immediately. With `--all`, app name arguments are ignored, but raw access tokens are still revoked.

## GitHub REST API

You can also revoke access tokens directly via the GitHub REST API.

[You can revoke access tokens by GitHub REST API.](https://docs.github.com/en/rest/credentials/revoke?apiVersion=2022-11-28#revoke-a-list-of-credentials)

```sh
curl -L \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/credentials/revoke \
  -d '{"credentials":["ghu_<REDACTED>"]}'
```

> [!NOTE]
> We Updated the guide at 2026-06-17. Previously, we misunderstood that the REST API doesn't support User Access Tokens and a client secret is required to revoke them.
> But actually, a client secret is unnecessary.

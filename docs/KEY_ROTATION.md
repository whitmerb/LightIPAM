# Key and certificate rotation

This runbook describes the current implementation. It does not provide an automated
multi-key/re-sealing migration. See [Security](SECURITY.md),
[Disaster recovery](DISASTER_RECOVERY.md), and [R12](ROADMAP.md#r12).

For published deployments, select the same Compose file/project used at install:

```sh
export COMPOSE_FILE=compose.release.yaml
```

Preserve the project and volume names. Use `compose.yaml` only for a source-build
installation; its development defaults are not operator-managed production secrets.

## What each credential actually controls

| Material | Used for | Rotation / revocation |
| --- | --- | --- |
| `APP_SECRET` | Fallback material from which the sealing key is derived when no valid dedicated encryption key is set | Changing it changes that fallback key, not database session/token validity. |
| `APP_ENCRYPTION_KEY` | Dedicated base64-encoded 32-byte sealing key | Protects TOTP, OIDC client secret, managed-CA key, webhook signing secrets and sealed authentication-flow cookies. Existing ciphertext needs the old effective key. |
| Session cookie / `sessions` rows | Opaque database-backed browser credentials with idle/absolute expiry | Use session revocation; rotating an application secret does not delete these rows. |
| API tokens | Independent bearer credentials stored as hashes | Revoke affected tokens explicitly; application-key rotation does not revoke them. |
| Managed CA / leaf files | App-to-agent mTLS trust and identities | Issue/deploy matching CA/leaf bundles and verify each peer. No online renewal or CRL is implemented. |

Store the effective encryption material separately from database backups. A dedicated
key prevents an `APP_SECRET` change from inadvertently changing the sealing key.
An invalid dedicated key currently falls back to `APP_SECRET` with a startup warning;
check configuration and startup logs rather than assuming a supplied key was accepted.

## Application master secret

If a valid dedicated `APP_ENCRYPTION_KEY` is already configured and unchanged,
changing `APP_SECRET` does not re-seal existing data and does not sign users out.
Generate a replacement through the operator's secret-management process, update the
Compose environment and recreate the app to load it:

```sh
docker compose up -d --force-recreate app
```

A plain restart does not apply changed Compose environment values. If the dedicated
key is absent, an `APP_SECRET` change is an encryption-key change: do not perform it
without the maintenance plan below. Use explicit session/token revocation when
responding to compromise, regardless of which application key changes.

## Encryption-key change: maintenance work, not a restart recipe

There is no complete built-in re-seal command or key ring. Do not replace the key
and rely on users to re-enroll after failures. TOTP challenge verification opens the
stored secret before trying a recovery code; a wrong key can therefore prevent that
login path too. Managed-CA startup also needs review before wrong-key testing against
valuable data: failed unsealing can lead to generating replacement CA state.

Prepare a controlled, rehearsed maintenance plan instead:

1. Back up the database and matching old effective key. Test recovery in an isolated
   copy and retain a working local administrator/recovery path.
2. Inventory all sealed values: TOTP enrollments, OIDC client secret, managed CA and
   webhook signing secrets. Choose an approved re-seal migration or deliberate
   re-provisioning plan for every class before changing configuration.
3. If re-provisioning is selected, complete necessary account recovery/enrollment
   preparation while the old key is still available. Re-entering only the OIDC
   secret does not repair TOTP, webhooks or the managed CA.
4. Quiesce application writes, apply the reviewed migration/configuration, recreate
   the app, then verify local and MFA/SSO login, CA identity, signed webhooks and
   agent connectivity in the isolated rehearsal before production execution.
5. Retain old encrypted backups with their corresponding key for the approved
   recovery period. Do not discard the old key merely because the app starts.

Implementing this migration and fail-closed recovery behavior is [R12](ROADMAP.md#r12),
not a feature already delivered by this documentation update.

## Managed CA and leaf rotation

Routine leaf renewal: use Settings → **Agent certificates** to issue/download the
agent and app bundles before expiry (default leaf lifetime: 30 days). Deploy each
bundle to its intended certificate volume. The agent hot-reloads its leaf files;
the app reads its dispatch client identity at startup and must be restarted after
file replacement. Keep private-key ownership/permissions correct on native Linux.

CA replacement: use **Rotate CA** only as a deliberate trust change, then issue and
distribute matching CA/agent/app bundles to every peer. Restart/recreate each
scanner-agent after replacing its trusted CA: the client-CA pool is loaded only at
startup even though its own leaf keypair hot-reloads. Restart the app after replacing
its dispatch client certificate/key or CA file. Old and new peers cannot communicate
until their trust/leaf sets match. After distribution and restarts, check agent
registration/diagnostics and a scan only within an authorized scope. Certificate
status/short TTLs are not a CRL revocation service.

The development `scanner-certs` command remains supported for local work; replacing
production trust with a newly generated development CA is not routine renewal.
Online agent-pull enrollment/renewal is still [R31](ROADMAP.md#r31).

## Compromise response

Use Security/Account session controls and revoke affected API tokens explicitly;
reset compromised passwords through Users & Roles. Decide whether CA/leaf or sealing
keys are affected and follow the appropriate maintenance path above. Preserve audit
records and matching recovery material. Rotating a key alone is not a complete
credential-revocation or incident-response procedure.

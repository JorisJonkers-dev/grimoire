# Grimoire owns its Accounts; jorisjonkers.dev is one linkable sign-in method

Grimoire keeps its own Accounts instead of trusting a platform forward-auth header, because people outside the estate must be able to play: an Admin invites them with a one-time Account Invite and they sign in with a Username and password. jorisjonkers.dev stays a first-class sign-in through OIDC, allowed only to estate users holding the Grimoire grant, and either method can be linked to the same Account so campaigns and Characters survive a switch. The route therefore no longer sits behind forward-auth; Grimoire runs its own sessions. Admin comes from the estate admin role at each OIDC sign-in, or from promotion by another Admin; losing the estate grant stops only the OIDC path, so locking someone out fully is an explicit Admin action. Internal passwords get optional TOTP, mandatory for an internal Account promoted to Admin.

## Considered Options

- Forward-auth only (the original design): no way in for people without an estate account.
- Any estate user auto-provisioned: widens access beyond the people the owner chose.
- One sign-in method per Account: simpler, but strands someone who later gets an estate account.

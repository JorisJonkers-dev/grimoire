# Grimoire owns its Accounts; jorisjonkers.dev is one linkable sign-in method

Grimoire keeps its own Accounts instead of trusting a platform forward-auth header, because people outside the estate must be able to play: an Admin invites them with a one-time Account Invite and they sign in with a Username and password. jorisjonkers.dev stays a first-class sign-in through OIDC, allowed only to estate users holding the Grimoire grant, and either method can be linked to the same Account so campaigns and Characters survive a switch. The route therefore no longer sits behind forward-auth; Grimoire runs its own sessions. Admin comes from the estate admin role at each OIDC sign-in, or from promotion by another Admin; losing the estate grant stops only the OIDC path, so locking someone out fully is an explicit Admin action. Internal passwords get optional TOTP, mandatory for an internal Account promoted to Admin.

## Considered Options

- Forward-auth only (the original design): no way in for people without an estate account.
- Any estate user auto-provisioned: widens access beyond the people the owner chose.
- One sign-in method per Account: simpler, but strands someone who later gets an estate account.

## Amendment: a Campaign Invite grants the OIDC path

People also arrive through a DM's Campaign Invite, not only an Admin's Account Invite. A jorisjonkers.dev identity that signs in while holding a valid pending Campaign Invite is accepted without the estate Grimoire grant: accepting the invite is the grant, and that identity may keep signing in through OIDC afterwards. The invite page therefore offers both sign-in methods to everyone. Any estate user without an invite or the grant is still refused, so access stays limited to people an Admin or a DM chose.

- Rejected, estate grant only: a friend with a jorisjonkers.dev login but no grant would be bounced to a Username and password for no reason.
- Rejected, hide SSO on link invites: the DM cannot tell whether the person behind a link has an estate login.

A jorisjonkers.dev identity is linked to an existing Account only from that Account's settings while signed in. An OIDC sign-in whose identity matches no Account and carries no pending invite is refused with a pointer to sign in with a Username and link it there; Grimoire never matches Accounts by email and never creates a second Account for someone who already has one.

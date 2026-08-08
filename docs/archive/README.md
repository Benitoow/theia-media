# Archive: the V2 planning documents

These five documents ran the V2 cycle, from the roadmap interview in July 2026 to
the last milestone shipping in August. **They describe a plan that is finished.**
Everything in them is either released in
[v2.0.0](https://github.com/Benitoow/theia-media/releases/tag/v2.0.0) or was
explicitly cut, so nothing here should be read as the current state of the
project.

They are kept rather than deleted because they hold reasoning that the code
cannot: why a milestone was sequenced where it was, what the alternatives were,
and what a contract looked like before the frontend consumed it. That is the same
reason [`DECISIONS.md`](../DECISIONS.md) is append-only.

| Document | What it was for |
|---|---|
| [`theia-v2-roadmap.md`](theia-v2-roadmap.md) | Product order, shared scope, blockers and milestone status |
| [`theia-v2-backend.md`](theia-v2-backend.md) | Backend queue and the API handoffs published before each frontend track started |
| [`theia-v2-frontend.md`](theia-v2-frontend.md) | Frontend work waiting behind each handoff, with visual and D-pad acceptance criteria |
| [`theia-v2-m3-discovery.md`](theia-v2-m3-discovery.md) | Scoping the series model before writing any of it |
| [`theia-v2-m4-discovery.md`](theia-v2-m4-discovery.md) | Scoping remote access, and ruling out every option with a control plane |

## What is current instead

Three documents govern the project, and they are maintained:

- [`../spec-fondatrice.md`](../spec-fondatrice.md) — what Theia is and refuses to be.
- [`../DECISIONS.md`](../DECISIONS.md) — every decision taken, with its reasoning.
- [`../design-system.md`](../design-system.md) — colour, type, spacing, motion, focus.

## The delivery split, and why it is over

V2 was built backend-first, in two tracks: a backend queue worked with Codex, and
a frontend queue that only started once the matching server contract was merged
and documented. The handoffs existed so that neither side had to reconstruct the
other's state from a conversation.

That worked, and it is no longer the shape of the work. Post-v2 changes are
ordinary changes: read the three documents above, make the change, record the
decision if it settles an argument.

# crackapack

Open virtual Magic: The Gathering boosters, fill your binders and trade with a crowd of NPCs.
No real money, no account, no server: one binary running on your own machine.

The idea comes from Holopocket, a Pokémon collecting site, adapted to Magic and turned into a
solo, local game. It is a personal project, published for anyone who wants to run it.

## Status

Pre-alpha. The simulation MVP exists; the following are not built yet: the UI,
the NPC simulation, catalogue sync, and data persistence.

## Try the simulation

The demo command runs a coin-rate sweep on a built-in synthetic dataset with no
files or network required:

```
go run ./cmd/crackapack demo
```

See [Simulation guide](docs/simulation.md) for the other commands (`simulate`,
`sweep`, `compare`) and how to read the sweep table.

## Scope

**In scope:**

- Booster opening at real pack collation rates
- Collection and binders
- An economy in coins, with a counter to sell and buy cards
- Hundreds of NPCs that own real collections and trade with you
- Quests, mini-games and local events

**Out of scope:** multiplayer, online server, accounts, playing games, deck building, real money,
ads.

The first catalogue covers 2005–2007: Ravnica: City of Guilds, Guildpact, Dissension, Coldsnap,
Time Spiral, Planar Chaos and Future Sight. Everything is in English, cards included.

## How it will run

`crackapack` is a single executable per OS. It serves its UI on `localhost` and opens your
browser. The first launch downloads the card catalogue and later launches keep it up to date. Card
images are downloaded on demand and cached.

## Documentation

- [Architecture](docs/architecture.md) — the core, its adapters, and how the online option stays
  open
- [Core features](docs/core-features.md) — boosters, energy, collection, the Counter, NPCs
- [Simulation guide](docs/simulation.md) — commands, config keys, metric names, how to read the
  sweep table
- [Retention](docs/retention.md) — quests, milestones, badges, mini-games, events
- [MTG vs Pokémon](docs/mtg-vs-pokemon.md) — what changes when adapting Holopocket to Magic
- [Specs review](docs/specs-review.md) — V1 scope, build order, risks, open questions
- [Architecture decisions](docs/adr/)

## Contributing

Conventions, commands and project rules for humans and coding agents are in
[AGENTS.md](AGENTS.md).

## Legal

crackapack is unofficial Fan Content permitted under the
[Fan Content Policy](https://company.wizards.com/en/legal/fancontentpolicy). Not approved/endorsed
by Wizards. Portions of the materials used are property of Wizards of the Coast. ©Wizards of the
Coast LLC.

Card data comes from [MTGJSON](https://mtgjson.com/) and [Scryfall](https://scryfall.com/).
crackapack is not affiliated with either.

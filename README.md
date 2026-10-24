# crackapack

Open virtual Magic: The Gathering boosters, fill your binders and trade with a crowd of NPCs.
No real money, no account, no server: one binary running on your own machine.

The idea comes from Holopocket, a Pokémon collecting site, adapted to Magic and turned into a
solo, local game. It is a personal project, published for anyone who wants to run it.

## Status

Pre-alpha. The simulation MVP exists, with NPCs that open boosters and supply the Counter. The
game is playable over several days in the browser, with `crackapack play`:

- You open the ten welcome boosters, then more for energy that recharges by itself, also while the
  game is closed. The game is saved after every action, so a restart picks it up where it was.
- It plays every set in scope, unlocked in stages as your binders fill.
- You browse your collection, one binder per set, with the progress overall, by rarity and by
  colour, and the collection stats on top.
- The game has coins and a Counter that stocks itself each day. You trade at it from the binders: a
  held card sells for its price, a missing one is bought when the Counter has a copy, and the
  duplicates above a playset sell in one click.
- The game's NPCs trade with you on a market. From an empty slot you ask around, and the NPCs that
  have the card answer with their prices; from a held card you list it, and the NPCs that want it make
  offers. You accept, counter once or decline, on the binder or on the Market page, and the header
  counts the answers waiting. Each NPC answers at the pace of its personality, some at once and some
  the next day.

Not built yet: quests, rewards and milestones, with milestones as named groups of cards such as the
Urzatron, the coin top-up for energy, catalogue sync, packaging, and a simulation of the market's
effect on the economy. See the [TODO](docs/todo.md).

## Open your first boosters

The play command serves the web client on localhost, on the built-in synthetic sets, with no
files or network required. The demo cards are drawn as placeholders:

```
go run ./cmd/crackapack play --demo
```

Then open the address it prints. The demo lets you try the market in a minute: on an empty slot of the
binder press Ask around, one NPC answers at once and others later, and after opening a booster you can
list its rare from the binder and get an offer at once. With real catalogues from `fetchdata`, pass
`--catalogue-dir ./catalogues --prices ./prices.json` instead of `--demo`. That plays every set of
the directory in scope, from Mirage on, in [stages](docs/core-features.md#stages): a few sets are
open at the start, and a binder filled to 90% unlocks three more. Add
`--sets RAV,TSP --supplementary TSB` to play a few sets, all open. The prices set what a booster
costs in energy, and without them every booster costs the same. The card
pictures are then downloaded from Scryfall the first time a card shows, and kept in your user cache
directory, see [ADR-0035](docs/adr/0035-card-pictures-are-served-from-a-local-cache.md). `fetchdata`
also downloads each set's icon next to its catalogue as `<CODE>.svg`; the home page shows it behind the
set's tile, and a set without one has a plain tile.

Your game is saved in a JSON file, which the command prints on startup: `crackapack/save.json` in
your user config directory, `save-demo.json` for the demo, or the path of `--save`. Delete the file,
or pass another `--save`, to start a new game. See
[ADR-0052](docs/adr/0052-the-game-is-saved-in-one-versioned-json-file.md).

## Try the simulation

The demo command runs a coin-rate sweep on a built-in synthetic dataset with no
files or network required:

```
go run ./cmd/crackapack demo
```

See [Simulation guide](docs/simulation.md) for the other commands (`simulate`,
`sweep`, `compare`, `tune`) and how to read the sweep table.

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
- [TODO](docs/todo.md) — what is left to build, the economy simulation in detail
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

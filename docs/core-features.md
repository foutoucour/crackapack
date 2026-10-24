# Core features

## Vision and scope

A local, solo application to open virtual Magic boosters, fill binders and trade with a crowd of
NPCs, with no real money. The model is Holopocket (Pokémon), adapted to Magic. It is a personal
project, with the code published on GitHub for anyone who wants to run it.

crackapack started as an online game with no NPCs, a direct adaptation of Holopocket. It became
solo and local, and the NPCs now stand in for the other players. Mechanics that only made sense
between real players were removed or deferred, see
[Specs review](specs-review.md#fixed-counter-instead-of-an-auction-market).

- **In scope:** booster opening at real rates, collection, a coin economy, the Counter, trades with
  NPCs, quests, mini-games, local events.
- **Out of scope:** multiplayer, online server, accounts, teams, binder contests, playing games,
  deck building, real money, ads.
- **V1 catalogue:** Ravnica: City of Guilds, Guildpact, Dissension, Coldsnap, Time Spiral, Planar
  Chaos, Future Sight (2005-2007). Option: Ninth and Tenth Edition.
- **Stack:** a Go core in a single binary, see [Architecture](architecture.md) and
  [ADR-0001](adr/0001-go-for-the-core.md).
- **Language:** everything in English, interface and cards (US printings).

## Legal and data

The published code remains Fan Content: Wizards' [Fan Content
Policy](https://company.wizards.com/en/legal/fancontentpolicy) and the [Scryfall API
terms](https://scryfall.com/docs/api) apply. No personal data is collected, so there is no GDPR,
Law 25 or terms of service.

- **Mandatory notice** in the README and the app footer: "crackapack is unofficial Fan Content
  permitted under the Fan Content Policy. Not approved/endorsed by Wizards…".
- **No Wizards trademarks in the project's own design:** no Magic logo, no packaging visuals. Packs
  are drawn in-house, and the project name contains neither "Magic" nor "Holopocket". The card
  pictures and the card back are Wizards' content, downloaded at run time and never stored in the
  repository, see [ADR-0036](adr/0036-the-card-back-is-downloaded-with-an-in-house-fallback.md).
- **Repository:** code only. No images, no Scryfall data and no MTGJSON data in the repository.
- **Data sources:** MTGJSON provides set contents, card identities and booster definitions, one
  file per set. Each card carries its Scryfall ID in `identifiers.scryfallId`, which links it to its
  Scryfall image. Prices come from the price snapshot, whose source is not chosen yet. MTGJSON also
  publishes a prices file, not checked for the V1 sets.
- **Code licence:** to be chosen.
- **Images:** downloaded from Scryfall into a local cache and shown as they are (no cropping,
  watermark or filter), except that the white corners outside the card's rounded border are masked.
- **Set icons:** the SVG of each set, downloaded from Scryfall into the same cache and drawn in the
  colour of a rarity, see [ADR-0065](adr/0065-a-set-icon-is-a-scryfall-svg-drawn-as-a-mask-in-the-colour-of-a-rarity.md).
- **Scryfall rate limit:** 50 to 100 ms between requests. Requests are made one image at a time.
- **Offline:** the network is required for the first sync and for catalogue updates. The game runs
  without a connection afterwards.

## Installation and saves

One player per installation: no account, no anti-cheat. Editing the database or the clock only
hurts yourself.

- **Installation:** download the binary for your OS and run it. It opens the game in your browser.
- **First launch:** downloads the catalogue, then generates the NPC population.
- **Welcome bonus:** 200 coins and 10 boosters, split evenly across the sets of the first
  [stage](#stages), the first sets taking the remainder.
- **Saves:** the game is one JSON file, copy it to back it up, see
  [ADR-0052](adr/0052-the-game-is-saved-in-one-versioned-json-file.md). The NPC population will need a
  SQLite file of its own. Export/import from the app is not built.
- **Seed:** the random seed is stored, to replay a simulation and debug it. A loaded game keeps its
  saved seed.
- **Prices:** a snapshot that comes with the catalogue. Prices stay fixed until a new snapshot is
  published, see [Specs review](specs-review.md#positioning-and-decisions).

## Boosters and draw rates

Each booster reproduces the composition and rates of the real booster of its set. This is the core
of the fun: the feeling of opening a Teferi or an Akroma.

The compositions below are the draft boosters defined by MTGJSON, checked on 2026-10-10 against
MTGJSON 5.3.0. MTGJSON is the source because Scryfall does not publish booster contents. All seven
are 15 cards, with no mythics (introduced in 2008). The simulation also loads sets with mythics, see
[ADR-0013](adr/0013-mythic-is-a-rarity-of-its-own.md).

| Set | MTGJSON sets | Non-foil slots | Foil |
| --- | --- | --- | --- |
| Ravnica | `RAV` | 11 common, 3 uncommon, 1 rare | About 1 pack in 5 (20.5%) |
| Guildpact | `GPT` | 11 common, 3 uncommon, 1 rare | About 1 pack in 5 (20.5%) |
| Dissension | `DIS` | 11 common, 3 uncommon, 1 rare | About 1 pack in 5 (20.5%) |
| Coldsnap | `CSP` | 11 common-or-basic, 3 uncommon, 1 rare | 22.5% of packs |
| Time Spiral | `TSP`, `TSB` | 10 common, 3 uncommon, 1 rare, 1 timeshifted | 22.5% of packs |
| Planar Chaos | `PLC` | 8 common, 3 colorshifted common, 2 uncommon, 1 rare, 1 colorshifted uncommon-or-rare | 22.5% of packs |
| Future Sight | `FUT` | 11 common, 3 uncommon, 1 rare | 22.5% of packs |

- **Ravnica block foils:** a foil replaces a common (15.7% of packs), an uncommon (4.3%) or the rare
  (1.4%). A few packs hold two foils.
- **Foils in the other four sets:** one foil replaces a common. It is drawn from the whole set,
  weighted by rarity: 60% common, 25% uncommon, 15% rare. In Time Spiral the weights are 50% common,
  25% uncommon, 12.5% rare and 12.5% timeshifted.
- **Timeshifted cards:** Time Spiral's timeshifted cards are a separate MTGJSON set, `TSB`. It holds
  122 cards of rarity "special", 121 of which are in boosters. Each is equally likely and can be
  foil.
- **Colorshifted slot:** Planar Chaos draws its uncommon-or-rare slot from 15 uncommons and 10
  rares, weighted 30 to 10, so one card in four is a rare.
- **Basic lands:** the only basics in non-foil packs are Coldsnap's 5 snow-covered basics, which
  share the common slots. Ravnica's foil sheet also contains 20 basics. They appear in packs as
  filler and do not count toward binders, see [Collection and binders](#collection-and-binders).
- **Future Sight:** the booster definition has no dedicated slot. Its cards come from the plain
  rarity sheets.
- **Colour balance:** the common sheets of Coldsnap, Time Spiral and Future Sight are flagged as
  colour-balanced in MTGJSON, and the draw engine must honour that.
- **Tournament packs:** MTGJSON also defines them for Ravnica and Time Spiral. Only draft boosters
  are used.
- **Sheets:** each slot draws from a sheet with a weight per card. Foil is tracked separately in the
  collection. Cards are linked to Scryfall images, see [Legal and data](#legal-and-data).
- **"Contents and rates" panel** before opening: cost, average value, rarities, "at least one per
  booster" probability.
- **Opening:** one booster or "Open all" (20 max), revealed card by card, rare last. Space turns
  the card and brings the next one up, Shift+Space skips to the end. Each card shows its reference
  value in coins, and a new rare or mythic comes with a burst of light and a glow.
- **More sets:** V1 is limited to the seven sets from Ravnica to Future Sight. Others come later,
  each with its own composition.
- **Draw:** a seeded random generator and a log of every opening. The player and the NPCs use the
  same engine.

## Energy

Energy is the only resource for opening boosters. It is free, recharges by itself and sets the
rhythm of sessions.

- **Recharge:** +1 every 10 minutes up to 10 (a full reserve in 1 h 40).
- **Savings:** with a full reserve, recharge continues at +1 every 30 minutes, capped at 200.
  Bonuses (chest, milestones, events) are added to it.
- **Spending:** the reserve first, then the savings.
- **Cost relative to the catalogue:** every V1 set is expensive in USD today. The cost follows the
  booster's average value relative to the catalogue median, not its absolute value.
- **Never purchasable**, not even with coins. A booster worth more than the 12-energy cap pays a
  coin fee on top of the energy, see
  [The coin fee on dear boosters](simulation.md#the-coin-fee-on-dear-boosters).

| Booster value / catalogue median | Energy cost |
| --- | --- |
| × 0.5 or less | 3 |
| × 1 | 5 |
| × 2 | 8 |
| × 4 and more | 12 |

Starting values, to be calibrated against the real average value of each set.

In the interactive game the energy recharges with the real clock, even while the page is closed,
and the ten welcome boosters are free and opened first, see
[ADR-0038](adr/0038-the-games-energy-recharges-by-the-clock.md). The game has no coin fee and no
coin top-up yet.

## Collection and binders

The collection is the player's goal: one binder per set, filled card by card. Cards are collected by
printing: the same card in two sets counts as two cards.

- **Set binder:** all cards of the set except basic lands, in collector number order, empty slots
  greyed out. The collection page filters it by rarity, by colour and by what the player can do with
  a card, and the three filters combine.
- **Cards from another list:** the cards that a set's boosters draw from another catalogue count in
  its binder, in a section of their own. Time Spiral's binder holds the 121 timeshifted cards that
  boosters can draw, and leaves out the one that no booster holds, since nobody could complete the
  binder with it.
- **Progress shown:** cards owned / total, by rarity and for the whole set, plus complete playsets
  (4 copies of a card, the reference unit in Magic).
- **Progress by colour:** the same count in eight groups, white, blue, black, red, green, gold (two
  colours or more), colourless and land. A card is in one group only. The colour is the MTGJSON
  `colors` of the one face the binder holds, which is the a side of a split or double-faced card, as
  the b side is not drawn by a booster and so is not in the binder, see
  [ADR-0037](adr/0037-a-binder-counts-the-pool-cards-its-boosters-draw.md). A card with two colours
  or more is gold. A card with no `colors`, devoid and colour-indicator cards included, is
  colourless. A non-basic land is a land whatever its colours. So a split card counts by its a half
  only: Fire // Ice is red, not gold. A group the set has no card in is left out, as a rarity is. The
  count is of the cards the boosters can draw, as for the binder.
- **Filters:** the rarity list and the colour list are panels of links, one row per rarity or colour
  with a bar of what the player holds. A row filters the binder, and the row in force clears its
  filter when clicked again. The bars always show the whole binder, so a filter never changes the
  progress.
- **Show filter:** a row of pills over the binder, each with the number of cards it keeps under the
  rarity and colour in force, that picks the cards to act on. *Duplicates* keeps the cards held above
  a playset, in both finishes together: the copies to sell or to trade. *At the Counter* keeps the
  cards the Counter has in stock, held or not, in a game with a Counter. *Available to trade* keeps
  the cards an NPC that trades holds, which is whether Ask around finds someone, in a game with a
  market. A choice that keeps nothing says so.
- **Finding a set:** the sets that can be opened are listed beside the binder, each entry filled up
  to the player's progress in it, with a search box above. Locked sets are folded into a closed
  section, which opens by itself when the binder in view is locked. On a narrow screen the list
  folds into the name of the set in view. With 107 sets, a row of buttons would push the binder off
  the screen.
- **Card tools:** the binder shows a held card with how many copies are held and nothing else. A
  click on a card, or on an empty slot, opens it big in a dialog, with its name and a row of icon
  buttons under it; a click outside the buttons, Close or Escape puts it away. Nothing depends on
  hovering, so the page works on a phone. Sell and Buy go to the Counter, and Trade asks the NPCs
  around or lists the card on the market. A sale of several copies asks the quantity, and a listing
  asks the price, in a panel under the icons. A request or a listing that is under way stays under
  its card in the binder, since its answers wait for the player. A binder card carries no set icon.
- **Overview:** at the top of the collection page, the cards held out of the cards of the unlocked
  sets, the binders complete (every card held) and the sets unlocked out of all of them. Locked sets
  do not count in the cards, since they cannot be played yet.
- **Unlock mark:** a binder that has reached the [unlock threshold](#stages) says so. The mark stays
  when cards are sold, as the unlocks do. The page shows stats only: it gives no reward.
- **Basic lands** are excluded from totals, progress and milestones. The Ravnica and Time Spiral
  basics only come as foils, so counting them would make a complete binder nearly impossible.
- **Master set:** every card in normal and foil, plus the variants present in the booster. The exact
  scope follows what MTGJSON describes for each set, at least foil, timeshifted and colorshifted.
- **Collection value** in US dollars (TCGplayer price) and in coins (the Counter's reference value).
- **Duplicates:** beyond a playset, they can be sold to the Counter or offered in a trade. The bulk
  sale keeps 4 copies by default, adjustable from 1 to 4.
- **Card page:** image, rarity, price, how many I own, which NPCs have it as a duplicate, price
  history once several snapshots exist.
- **Wish list:** 20 cards max, feeds alerts and the search for trades.
- **Showcase:** a few chosen cards, visible on the profile.
- **Custom binders:** a free theme (one colour, one artist, one creature type).

## Stages

The sets open up as the player progresses, so a new player starts with a few sets instead of more
than a hundred.

- **Order:** the sets rank from the quickest binder to fill to the slowest, by the energy a player
  spends to reach the unlock threshold, see
  [ADR-0053](adr/0053-the-stages-rank-the-sets-by-the-energy-to-the-unlock-threshold.md).
- **Stages:** the first `unlock.start_sets` sets of the order are the first stage, open from the
  start. The rest are cut into `unlock.stages` stages of equal size.
- **Unlocks:** a binder that reaches `unlock.binder_threshold` of its cards unlocks
  `unlock.sets_per_unlock` sets of the next stage, drawn at random, once, see
  [ADR-0054](adr/0054-a-binder-past-the-threshold-unlocks-random-sets-of-the-next-stage-once.md).
- **Locked sets** show greyed on the home page and cannot be opened. Their binders show in the
  collection, marked as locked.
- **Scope:** the stages hold every set of the product's scope, see
  [ADR-0039](adr/0039-the-product-covers-the-sets-from-mirage-under-60-dollars-a-booster.md). A game on
  a list of sets has no stage.

The current values of the keys are in the [tuning log](tuning-log.md).

## Economy: coins, sources and sinks

Coins flow between the player and the Counter, and on the market between the player and the NPCs.
NPCs hold no coins, see [Specs review](specs-review.md#positioning-and-decisions): the coins an NPC
pays for a card enter through the market's account, see
[ADR-0060](adr/0060-an-npc-pays-coins-for-a-card-on-the-market.md). The main settings are the
Counter's rates and the rewards.

| Flow | Direction for the player | Starting value |
| --- | --- | --- |
| Welcome bonus | Source | 200 coins |
| Daily reward | Source | 100 coins / day |
| Sales to the Counter | Source | 60% of the reference value |
| Sales to NPCs on the market | Source | Up to the reference value less the NPC's margin |
| Quests, milestones, mini-games | Source | See [Retention](retention.md) |
| Coin top-ups in trades, and purchases from NPCs on the market | Sink | Difference in reference value, or the NPC's asking price |
| Purchases from the Counter | Sink | 120% of the reference value |
| Foils from the Counter | Sink | `counter_foil_pct` of the card's reference value, then 120% |

- Coins have **no monetary value**.
- **Coin ↔ USD rate:** to be set, the whole economy depends on it.
- Track the player's money supply to tune rewards and prices.
- Recycling duplicates into coins is dropped for now, see
  [Specs review](specs-review.md#fixed-counter-instead-of-an-auction-market).

## The Counter

The Counter buys and sells cards at fixed prices, with no negotiation. It is the only place to turn
coins into cards without an NPC.

- **Instant sale:** at 60% of the reference value, paid at once.
- **Purchase:** at 120% of the reference value, if the card is in stock.
- **Reference value:** the card's TCGplayer USD price, converted at the fixed coin ↔ USD rate. The
  price comes from the current price snapshot.
- **Stock:** fed by the sales of the player and of the NPCs. The game provides an initial stock at
  startup.
- **Bulk sale:** "sell everything above the playset" in one click.

In the game, the Counter's rules live in `play.Game`, and the player trades from the binders, on
the collection page, with no page of its own:
- **Coins:** a new game starts with the welcome bonus. A save from before coins existed gets it too.
- **Selling:** any copy, the only one included, or every copy above a playset of the cards of a set's
  binder, foils kept longest. A sale never locks a set again, see
  [ADR-0054](adr/0054-a-binder-past-the-threshold-unlocks-random-sets-of-the-next-stage-once.md).
- **Buying:** one copy of a card in stock, missing or not, if a set that is unlocked holds it in its
  binder. A purchase that fills a binder past the threshold unlocks sets, as an opening does.
- **What the player sees:** the two prices and whether the Counter has a copy, never how many. A
  card out of stock tells when the Counter last had it, or that it never did, see
  [ADR-0056](adr/0056-the-counter-remembers-when-it-last-had-a-card.md).
- **Stock:** the initial stock, then what the player sells and what the Counter's supply and the NPCs
  bring each day, also while the game is closed, see
  [ADR-0055](adr/0055-the-counter-catches-up-on-whole-days-up-to-a-configured-cap.md). The game plays
  the product's base config with an overlay that makes the Counter scarce: no rare or mythic at the
  start, and a shelf that does not keep cards forever, see
  [ADR-0057](adr/0057-the-game-plays-the-products-base-config.md).
- **Prices:** from `--prices`. Without a snapshot every card is worth the fallback price, one cent, as
  a card the snapshot lacks is in the simulation, so every card costs one coin and pays one.
- **Refusals** change nothing: not enough coins, out of stock, nothing to sell, a locked set, an
  unknown card, or a treasury that cannot pay. The page says which, with a link back to the slot.
- **Cards on the market:** the Counter does not buy a copy held for a listing or a counter on the
  market, see [The market](#the-market). Selling the duplicates keeps those copies.
- **In the binder:** the coins show in the header next to the energy. A held card has a Sell
  button with the price of a copy, per finish, and a quantity when more than one copy is held. It
  has a Buy button too, as a slot with no copy does, since the player may be building a playset. A sale
  that empties the slot, with no copy left in either finish, asks first on a page of its own, so
  that no script is needed. The binder offers Sell duplicates with the copies and the total it
  pays, and says so when there is nothing above a playset. A slot shows the card's name, its price and
  a Buy button, or why it cannot be bought: the Counter has no copy and when it last had
  one ("last in stock 5 minutes ago"), it never had one ("never seen at the Counter"), or it does
  not trade the card. A locked set's binder only sells. The result of a purchase or a sale shows on
  the binder it returns to, with the sets it unlocked.

Both prices derive from one reference, so buying a card and selling it back loses half of its
value at the default rates, and never gains at any rate the configuration accepts. The 60% and 120%
rates are starting values to calibrate.

## Trades with NPCs

Trades let the player complete a binder by negotiating with NPCs.

1. An NPC proposes a trade when its wish list matches the player's duplicates.
2. The player can also propose: up to 10 cards, plus coins, against cards from an NPC. The coins
   always come from the player.
3. The NPC accepts if it receives at least the reference value of what it gives, plus a margin that
   depends on its archetype. The margin is never negative, so an NPC never gives more value than it
   gets. Values are the Counter's reference values.
4. A counter-offer is possible on both sides, once: see [ADR-0012](adr/0012-an-npc-counters-once.md).
   An NPC gives only the surplus it holds on its shelf, see
   [ADR-0011](adr/0011-npcs-hold-surplus-on-a-shelf.md).
5. The transfer is atomic: everything goes through or nothing does.

- **Expiry** of an unanswered offer: 3 days by default, `trades.offer_expiry_days`, the day it is made
  included.
- **Trade help:** "which NPCs have this card as a duplicate", NPCs whose duplicates match my wish
  list.
- **History** of trades received, sent and concluded.
- A trade that removes the last copy of a card can cost a collection milestone, see
  [Retention](retention.md#collection-goals-and-badges).

The rules above are those of the simulation's Exchange. The game trades on a market instead.

### The market

In the game the player starts trades, as on Reverb or eBay, see
[ADR-0058](adr/0058-the-player-trades-on-a-market-of-requests-and-listings.md). The rules live in
`domain.Market`, and `play.Game` exposes them: `AskAround`, `Withdraw`, `List`, `Unlist`,
`AcceptAnswer`, `DeclineAnswer`, `CounterAnswer` and `Market`, the view. The pages are in
`internal/web` and hold no rule: they show what the view returns and call these methods.

- **Hunt:** the player asks around for a card. Every NPC that has it on its shelf answers with its
  asking price, the card's reference value plus its margin, in coins. Asking around for a card of a
  locked set is refused, as buying it is.
- **Sell:** the player lists one copy of a card at an asking price, and the copy is reserved until it
  is sold or unlisted. The NPCs that wish for the card, and the speculators that lack it, offer the
  cards of their shelf the player lacks, worth no more than the card once their margin is paid, or
  else coins: the card's value less their margin, never above the asking price.
- **Answers** come after the NPC's response delay, set by its archetype: some at once, some hours or
  a day later, see [ADR-0059](adr/0059-an-npc-answers-on-the-market-after-the-delay-of-its-archetype.md).
  The game delivers them when it loads and on every market call, with the instant they fell due, so
  a player who comes back later finds them waiting.
- **Accept** trades at once, atomically, and ends the request or the listing with its other answers.
  **Decline** ends the answer. **Counter** once: on an asking price the player offers coins and cards,
  which are reserved until the reply; on an offer it asks for coins. The NPC replies once, after its
  delay, by the acceptance rule and its traits' vetoes. A refused counter ends the answer.
- **Lapse:** an answer the player leaves alone ends `trades.offer_expiry_days` after it came. A
  countered answer waits for its reply.
- **What the player sees:** the requests, the listings, the answers with who made them and their
  terms, how many NPCs asked have still to answer a request but not who, and the latest answers that
  ended: accepted, refused, lapsed, or failed because the NPC no longer had its card.
- **In the binders:** on a slot of an open set, held or not, next to the Counter's price, an Ask
  around button. Once asked, the slot says how far the NPCs have answered ("3 NPCs asked, no answer yet",
  "3 NPCs asked, 1 still to answer"), then shows each answer with who made it, its personality and its
  asking price, and Accept, Counter and Decline buttons, with Stop asking. On a held card, per
  finish, a List on the market form with the asking price, which starts at what the Counter sells the
  card for since the game exposes no other reference. A listed card shows "Listed at 100 coins", its
  offers with the same three buttons, and Unlist; a card held for a counter says so.
- **Counter:** a page of its own, `/counter-offer`, reached from an answer. On an asking price it
  takes coins and, from the spare cards, the copies above a playset that the market does not already
  hold, the cards to add; on an offer it takes coins only. A counter waiting for its reply shows on
  the slot and the market page with the time to the reply.
- **The Market page:** `/market` lists every request, listing and answer, newest first, with the same
  buttons, and the latest results: bought, sold, refused, lapsed or failed, with how long ago. The
  header links to it from every page in a game with a market, with the number of answers waiting.
  After an accept the page it returns to says what was bought or sold, and the sets it unlocked.
  Refusals keep the player where they were with a message: nobody has the card, already asking, on
  the market already, not enough coins, held for the market, and the others of
  [the market's errors](#the-market).
- **The NPCs:** the game's crowd, its size and how long it keeps its surplus are in the
  [tuning log](tuning-log.md). Its NPCs always trade, whatever `trades.enabled`, the simulation's
  switch, says. They open the sets the player can open,
  see [ADR-0063](adr/0063-the-npcs-of-the-game-open-the-sets-the-player-can-open.md), and a new
  game's NPCs have played most of a hold already, see
  [ADR-0064](adr/0064-a-new-games-npcs-have-most-of-a-hold-behind-them.md). They are saved with the
  game, see [ADR-0061](adr/0061-the-npcs-are-saved-in-the-games-file-in-a-compact-form.md).

## NPCs and simulation

A crowd of NPCs with real collections makes the game feel alive. They open boosters with the same
engine as the player, so supply follows the real rates.

| Archetype | Behaviour |
| --- | --- |
| Targeted collector | One set, colour or artist; proposes trades for what they are missing |
| Playset hunter | Wants commons and uncommons by 4 |
| Speculator | Demanding in trades: asks for a margin on top of reference value |
| Casual | Opens boosters, accepts fair-value trades, sells surplus to the Counter; main source of supply |
| Whale | Opens far more boosters than the others, goes for rares and foils, of every colour |
| Stingy (black) | Rarely proposes and always answers; demands more margin for the rarer cards it gives |
| Fish (green) | Passive and friendly: proposes often at no margin, chases pricey and shiny cards, supplies duplicates |
| Ego (blue) | Proud: proposes again after a decline, refuses every counter, demands more for cards it held longer |
| Gambler (red) | Tilts: each decline in a row raises how often it proposes and lowers its extra margin |
| Archivist (white) | Opens the oldest loaded set and trades fairly |
| Ego Stingy (U/B) | Miser-plus-ego mix: adds stingy rarity margin and ego endowment margin; vetoes every player counter |
| Archivist Ego Stingy (W/U/B) | Three-trait mix: archivist wish-scoring (oldest sets first) on top of ego and stingy margin adds |

The five personalities are tied to the five colours and are traits that can mix in one archetype.
Traits mix in any order: their margin adds accumulate on top of the base margin, capped by
`npcs.max_margin_pct`, and their wish scores add. See [NPC crowd](simulation.md#npc-crowd).

Margins are configuration, not code. The starting values are 0% for a casual and 10 to 20% for a
speculator. The others are set through simulation, see
[Simulating the economy](specs-review.md#simulating-the-economy).

An archetype tied to Magic colours wishes only for cards in those colours.

- **Population:** 300 to 1,000 NPCs, adjustable. Each has a name, avatar, archetype, activity
  rhythm and wish list. The activity rhythm sets how many boosters it opens.
- **Real collections**, stored as quantities (NPC, printing, foil, quantity). NPCs sell surplus
  cards to the Counter, which bounds the database.
- **What an NPC does in a tick:** opens boosters, sells surplus to the Counter and proposes trades.
  It holds no coins: what it pays a player on the game's market comes from the market's account, see
  [ADR-0060](adr/0060-an-npc-pays-coins-for-a-card-on-the-market.md). The simulation runs supply and trades, see
  [NPC crowd](simulation.md#npc-crowd) and [Trades](simulation.md#trades).
- **Time step:** one tick every 10 minutes of game time.
- **Offline catch-up:** the whole days gone by are simulated when the game loads or the Counter is
  used, capped at `catch_up_days`, see
  [ADR-0055](adr/0055-the-counter-catches-up-on-whole-days-up-to-a-configured-cap.md). The game's
  NPCs trade on the market, see [The market](#the-market).
- **Coin ledger:** double-entry, covers the player, the Counter and the market. NPCs have no account.
- **Volume:** 500 NPCs × 10 boosters a day ≈ 75,000 cards drawn a day, stored in SQLite.

## Deferred: auction house

Not in V1. The Counter replaces it, see
[Specs review](specs-review.md#fixed-counter-instead-of-an-auction-market). The design is kept in
case it comes back after V1. Bringing it back means giving NPCs coins and budgets again.

- **Listing:** one card, a starting bid, a duration from 30 minutes to 24 hours (4 hours by
  default), 10 active listings max for the player.
- **NPC bids:** maximum price = reference value × interest in the card (wish list, archetype) ×
  random noise, within budget.
- **Coins locked** while the player's bid leads.
- **Anti-sniping:** a bid in the last 2 minutes extends the auction by 2 minutes, and NPCs can
  outbid.
- **Seller tax:** 5% of the final price. A 50-coin fee promotes a listing.
- **Cancellation** while no bid has been placed.
- **Filters:** set, rarity, colour, price, ending soon, cards from my wish list.
- **Offline:** auctions continue during the player's absence and resolve in the catch-up.

## Open questions

- [x] Solo and local project, code on GitHub.
- [x] Trades with NPCs that have real collections.
- [x] No teams or contests.
- [x] V1 catalogue: Ravnica to Future Sight (7 sets).
- [x] Language: everything in English, TCGplayer USD prices.
- [x] Collection by printing.
- [x] Project name: crackapack.
- [x] Does MTGJSON cover the 2005-2007 boosters, timeshifted slots included? Yes, see
  [Boosters and draw rates](#boosters-and-draw-rates).
- [x] Basic lands do not count toward binders, progress or milestones, see
  [Collection and binders](#collection-and-binders).
- [x] NPCs hold no coins, and never give more reference value than they receive, see
  [Specs review](specs-review.md#positioning-and-decisions).
- [ ] Ninth and Tenth Edition in V1? Their MTGJSON booster definitions are not checked yet.
- [ ] Fixed coin ↔ USD rate.
- [ ] Reference value of a card that has no TCGplayer price.
- [ ] Default number of NPCs.
- [x] Expiry duration of a trade offer: 3 days, see [Trades](simulation.md#trades).
- [ ] Code licence.

Sources: [Holopocket](https://holopocket.fr), [Fan Content
Policy](https://company.wizards.com/en/legal/fancontentpolicy), [Scryfall
API](https://scryfall.com/docs/api), [MTGJSON](https://mtgjson.com/).

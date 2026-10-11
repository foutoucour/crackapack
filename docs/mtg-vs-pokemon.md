# MTG vs Pokémon

## Point-by-point comparison

Magic is bigger, older and more extreme in value than Pokémon. The Holopocket model carries over,
but the economy and the collection must be recalibrated.

| Aspect | Pokémon (Holopocket) | MTG | Impact on the project |
| --- | --- | --- | --- |
| Catalogue | 130 FR sets since 2014 | Over a hundred main sets since 1993, over 30,000 unique cards | Choose which sets can be opened; a much bigger database |
| Cards per booster | 5 | 14 to 15 | Collection about 3 times faster, more duplicates; energy and quests to recalibrate |
| Booster types | One per set | Draft, Set, Collector, Play depending on the era | We keep the draft booster |
| Rarities | A dozen labels that change across eras | Common, uncommon, rare, mythic (+ special treatments) | Simpler to model and display |
| Variants | Normal, reverse | Foil, etched, showcase, borderless, extended art, retro frame, serialized… | Huge master set; scope to fix with MTGJSON |
| Reprints | Moderate | The same card in dozens of sets | Collection by printing |
| Value gap | An old booster is worth tens of times a recent one | An Alpha/Beta booster is worth thousands of dollars | Logarithmic cost and energy savings |
| Collection unit | 1 copy per card | Playset of 4 copies | Dedicated playset milestones, bulk sale that keeps 4 copies |
| Booster composition | Simple slots | Bonus sheets (The List, Special Guests…), old print sheets | Modelled through MTGJSON |
| Third-party IP | No | Universes Beyond (Lord of the Rings, Marvel…) | Excluded for now |
| Metadata | Type, HP, attacks | Colour, mana cost, type, subtype, plane, artist | Natural mini-games, filters and badges |
| Language | FR cards | EN cards (US printings) | One version per card |

## What makes the project harder

The three major difficulties are booster composition, the value scale and the explosion of
variants.

1. **Booster composition:** 14 to 15 cards, several types per set, bonus sheets, old collation by
   print sheets. Impossible to reproduce faithfully without MTGJSON. MTGJSON defines the draft
   booster of all seven V1 sets, see [Core features](core-features.md#boosters-and-draw-rates).
2. **Value scale:** from recent boosters at a few dollars to 1993 sets at several thousand. Energy
   and coins must hold across the whole range.
3. **Variants:** every recent set multiplies the treatments, which inflates the master set and the
   database.
4. **Volume:** more cards per booster, more sets, more duplicates to handle and display.
5. **Double-faced cards:** two images and an adapted card page.
6. **Rights:** Universes Beyond adds rights holders besides Wizards.

## What makes the project simpler

Magic has a more regular structure and richer data, which make display and retention easier.

- **Stable rarities:** 4 levels plus a few treatments, the same since 2008.
- **One language:** everything in English, US printings.
- **Rich metadata:** colour, cost, type, plane and artist feed Cartodle, The Grid, Connections and
  the filters.
- **Ready-made themes:** 5 colours, 10 guilds, planes and planeswalkers for badges and custom binders.
- **Playsets:** a natural reason to keep duplicates and trade, absent from Holopocket.
- **Obvious exclusions:** basic lands, tokens, promo cards and digital sets (Alchemy) are not
  collected.

## Decisions taken as a result

| Topic | Decision |
| --- | --- |
| Format | Solo, local, code on GitHub; the Counter and trades with NPCs |
| Collection | By printing |
| V1 catalogue | Ravnica, Guildpact, Dissension, Coldsnap, Time Spiral (with its timeshifted cards), Planar Chaos, Future Sight (15-card boosters, no mythics) |
| Sets outside the catalogue | Openable only during events |
| Energy | Recharge up to 10, savings up to 200, cost relative to the catalogue median |
| Variants and master set | Scope set by what MTGJSON describes (foil, timeshifted, colorshifted) |
| Playsets | Dedicated milestones; the bulk sale keeps 4 copies by default |
| Excess duplicates | The Counter: sells at 60%, buys at 120%, stock fed by the player and the NPCs |
| Badges | By block, guild, colour and great figure (Teferi, Akroma…) |
| Universes Beyond | Excluded |
| Language and prices | Everything in English, TCGplayer USD prices |
| Daily reset | Midnight UTC |

Details in [Core features](core-features.md) and [Retention](retention.md).

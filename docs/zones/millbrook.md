# Zone: Millbrook

**ID**: `millbrook` | **Reset**: lifespan, 6h | **Rooms**: 30

## Goals

Millbrook is where every player begins. It should feel like a real town — lived-in,
purposeful, slightly worn around the edges. Players should be able to orient
themselves quickly, find what they need (equipment, information, safety), and feel
curious about what lies beyond the east gate.

## Design Principles

- Fully peaceful. No combat anywhere in the zone.
- NPCs populate every district but don't need much mechanical depth — their
  presence signals life, not challenge.
- Shops are the primary mechanical function: give players a baseline kit before
  they head into the Darkwood.
- Geography should feel coherent: the square is the hub, districts branch from it,
  and the east gate is the obvious point of departure.

## Districts

| District      | Rooms | Notes                                              |
|---------------|-------|----------------------------------------------------|
| Town center   | 5     | square, market, town hall, well, notice board      |
| Inn           | 4     | common room, cellar, guest hall, guest room        |
| Smithy        | 2     | forge, yard                                        |
| Shop row      | 3     | general store, apothecary, alley                   |
| Gates         | 4     | north road, south road, south gate, east gate      |
| Residential   | 4     | cottage lane, residential, riverside, docks        |
| Outskirts     | 4     | fisherman's shack, cemetery path, cemetery, mausoleum |
| Stable        | 1     | stable                                             |
| Other         | 3     | remaining connector rooms                          |

## Loot

Equipment is currently placed in containers throughout the zone (weapon rack in the
smithy, armor stand, shop shelves). The strongbox in the smithy is locked; the key
exists as `millbrook-strongbox-key` but has no vendor to sell it yet.

## TODO

- **Vendor system**: NPCs should be able to sell items. The shopkeeper, blacksmith,
  and apothecary are all designed with shops in mind but cannot transact yet.
- **Strongbox key**: Once vendors exist, the strongbox key should be sold by the
  shopkeeper. Currently it would need to be placed somewhere accessible in the world.
- **Notice board**: The notice board room exists but has no mechanical function yet.
  Could eventually display zone announcements, wanted postings, or quest hooks.

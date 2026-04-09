#!/usr/bin/env python3
"""Convert CircleMUD parsed object JSON into go-mud Object assets.

Reads object definitions from _output/obj/ and zone placement data from _output/zon/.
Writes:
  - Object assets to circlemud/objects/<zone-slug>/
  - Updates room files in circlemud/rooms/ with object_spawns

Reuses zone metadata from import_circlemud_rooms.py (same slugify, same zone list).
"""

import json
import os
import re
import sys
import urllib.request

BASE_URL = "https://raw.githubusercontent.com/isms/circlemud-world-parser/master/_output"
OUTPUT_DIR = os.path.join(os.path.dirname(__file__), "..", "circlemud")

ZONE_NUMS = [
    0, 9, 12, 15, 25, 30, 31, 33, 35, 36, 40, 50, 51, 52, 53, 54,
    60, 61, 62, 63, 64, 65, 70, 71, 72, 79, 120, 150, 186,
]

# CircleMUD wear flag note -> our wear slot name.
# WEAR_TAKE is handled separately (absence -> "immobile" flag).
WEAR_SLOTS = {
    "WEAR_FINGER": "finger",
    "WEAR_NECK": "neck",
    "WEAR_BODY": "body",
    "WEAR_HEAD": "head",
    "WEAR_LEGS": "legs",
    "WEAR_FEET": "feet",
    "WEAR_HANDS": "hands",
    "WEAR_ARMS": "arms",
    "WEAR_SHIELD": "shield",
    "WEAR_ABOUT": "about",
    "WEAR_WAIST": "waist",
    "WEAR_WRIST": "wrist",
    "WEAR_WIELD": "wield",
    "WEAR_HOLD": "hold",
}

# CircleMUD effect flags we map to our object flags.
EFFECT_FLAGS = {
    "INVISIBLE": "invisible",
    "NODROP": "no_drop",
    "NOSELL": "no_sell",
}

# CircleMUD effect flags we preserve but don't map.
EFFECT_SKIP = {"NORENT", "NODONATE", "NOINVIS", "MAGIC", "BLESS", "GLOW", "HUM",
               "ANTI_GOOD", "ANTI_EVIL", "ANTI_NEUTRAL",
               "ANTI_MAGIC_USER", "ANTI_CLERIC", "ANTI_THIEF", "ANTI_WARRIOR"}

# CircleMUD affect location note -> our perk modifier key.
AFFECT_KEYS = {
    "STR": "core.stats.str",
    "DEX": "core.stats.dex",
    "INT": "core.stats.int",
    "WIS": "core.stats.wis",
    "CON": "core.stats.con",
    "CHA": "core.stats.cha",
    "HIT": "core.resource.hp.max",
    "AC": "core.combat.ac.flat",
    "HITROLL": "core.combat.attack.flat",
    "DAMROLL": "core.damage.all.flat",
}

# CircleMUD weapon type (values[3]) -> damage verb for reference.
WEAPON_TYPES = {
    0: "hit", 1: "sting", 2: "whip", 3: "slash", 4: "bite",
    5: "bludgeon", 6: "crush", 7: "pound", 8: "claw", 9: "maul",
    10: "thrash", 11: "pierce", 12: "blast", 13: "punch", 14: "stab",
}

# Container closure bitvector flags.
CONTAINER_CLOSEABLE = 1
CONTAINER_PICKPROOF = 2
CONTAINER_CLOSED = 4
CONTAINER_LOCKED = 8


def slugify(name):
    return name.lower().replace("'", "").replace(",", "").replace(".", "").replace("  ", " ").replace(" ", "-").strip("-")


def collapse_newlines(s):
    return re.sub(r"\s+", " ", s).strip()


def fetch_json(path):
    url = f"{BASE_URL}/{path}"
    with urllib.request.urlopen(url) as resp:
        return json.loads(resp.read())


# ---------------------------------------------------------------------------
# Zone metadata (shared with room/mob importers)
# ---------------------------------------------------------------------------

def build_zone_info():
    zones = {}
    for zn in ZONE_NUMS:
        data = fetch_json(f"zon/{zn}.json")
        for z in data:
            slug = slugify(z["name"])
            zones[z["id"]] = {
                "id": z["id"],
                "name": z["name"],
                "slug": slug,
                "bottom_room": z.get("bottom_room", 0),
                "top_room": z.get("top_room", 0),
            }
    return zones


def build_vnum_to_zone(zones):
    v2z = {}
    for z in zones.values():
        for vnum in range(z["bottom_room"], z["top_room"] + 1):
            v2z[vnum] = z["slug"]
    return v2z


def vnum_to_id(vnum, v2z, zones):
    slug = v2z.get(vnum)
    if slug is None:
        zone_num = vnum // 100
        slug = zones[zone_num]["slug"] if zone_num in zones else "circlemud"
    return f"{slug}-{vnum}"


def obj_id(vnum, v2z, zones):
    return vnum_to_id(vnum, v2z, zones)


def room_id(vnum, v2z, zones):
    return vnum_to_id(vnum, v2z, zones)


# ---------------------------------------------------------------------------
# Object conversion
# ---------------------------------------------------------------------------

def convert_object(src, zone_slug, v2z, zones, known_obj_vnums):
    flags = []
    perks = []

    type_note = src.get("type", {}).get("note", "OTHER")
    values = src.get("values", [0, 0, 0, 0])
    # Pad to 4 elements if short.
    while len(values) < 4:
        values.append(0)

    # --- Wear slots ---
    wear_notes = [w.get("note", "") for w in src.get("wear", [])]
    has_take = "WEAR_TAKE" in wear_notes

    slots = []
    for wn in wear_notes:
        slot = WEAR_SLOTS.get(wn)
        if slot:
            slots.append(slot)

    if slots:
        flags.append("wearable")
    if not has_take and not slots:
        flags.append("immobile")

    # --- Effect flags ---
    skipped_effects = []
    for eff in src.get("effects", []):
        note = eff.get("note", "")
        if note in EFFECT_FLAGS:
            flags.append(EFFECT_FLAGS[note])
        elif note in EFFECT_SKIP:
            skipped_effects.append(note)

    # --- Type-specific handling ---
    unused = {}

    if type_note == "WEAPON":
        dice = values[1]
        sides = values[2]
        if dice > 0 and sides > 0:
            perks.append({"type": "grant", "key": "attack", "arg": f"{dice}d{sides}"})
        wtype = values[3]
        if wtype in WEAPON_TYPES:
            unused["weapon_type"] = WEAPON_TYPES[wtype]

    elif type_note == "ARMOR":
        ac = values[0]
        if ac != 0:
            perks.append({"type": "modifier", "key": "core.combat.ac.flat", "value": ac})

    elif type_note == "LIGHT":
        burn_time = values[2]
        if burn_time != 0:
            perks.append({"type": "grant", "key": "light"})
        if burn_time > 0:
            unused["burn_time_hours"] = burn_time

    elif type_note == "CONTAINER":
        flags.append("container")
        capacity = values[0]
        if capacity > 0:
            unused["capacity"] = capacity
        cflag = values[1]
        if cflag & CONTAINER_CLOSEABLE or cflag & CONTAINER_CLOSED or cflag & CONTAINER_LOCKED:
            closure = {"closed": bool(cflag & CONTAINER_CLOSED)}
            if cflag & CONTAINER_LOCKED:
                key_vnum = values[2]
                if key_vnum > 0 and key_vnum in known_obj_vnums:
                    lock = {
                        "key_id": obj_id(key_vnum, v2z, zones),
                        "locked": True,
                    }
                    if cflag & CONTAINER_PICKPROOF:
                        lock["pickproof"] = True
                    closure["lock"] = lock
                else:
                    # Key object missing or is the vnum-0 placeholder.
                    # Our Lock requires key_id, so store the original
                    # state for future use.
                    unused["keyless_lock"] = True
                    if key_vnum > 0:
                        unused["keyless_lock_missing_key"] = key_vnum
                    if cflag & CONTAINER_PICKPROOF:
                        unused["keyless_lock_pickproof"] = True
            unused["closure"] = closure

    # --- Affect fields -> perks ---
    for aff in src.get("affects", []):
        note = aff.get("note", "")
        val = aff.get("value", 0)
        key = AFFECT_KEYS.get(note)
        if key and val != 0:
            perks.append({"type": "modifier", "key": key, "value": val})

    # --- Extra descriptions ---
    # In CircleMUD, the extra_desc whose keywords match the object aliases
    # serves as the detailed description (shown on "look <item>"). Pull it
    # out as detailed_desc and keep the rest as extra_descs.
    aliases_set = set(a.lower() for a in src.get("aliases", []))
    detailed_desc = ""
    extra_descs = []
    for ed in src.get("extra_descs", []):
        keywords = ed.get("keywords", [])
        desc = collapse_newlines(ed.get("desc", ""))
        if not keywords or not desc:
            continue
        ed_kw_set = set(k.lower() for k in keywords)
        if not detailed_desc and ed_kw_set & aliases_set:
            detailed_desc = desc
        else:
            extra_descs.append({"keywords": keywords, "description": desc})

    # --- Build asset ---
    vnum = src["id"]
    oid = obj_id(vnum, v2z, zones)

    obj = {
        "aliases": src.get("aliases", []),
        "short_desc": src.get("short_desc", "").strip(),
    }

    long_desc = src.get("long_desc", "").strip()
    if long_desc:
        obj["long_desc"] = long_desc

    if detailed_desc:
        obj["detailed_desc"] = detailed_desc
    elif long_desc:
        # No matching extra_desc; fall back to long_desc so "look <item>"
        # shows something rather than blank.
        obj["detailed_desc"] = long_desc

    if flags:
        obj["flags"] = flags
    if slots:
        obj["wear_slots"] = slots
    if perks:
        obj["perks"] = perks
    if extra_descs:
        obj["extra_descs"] = extra_descs

    # --- circlemud_unused: preserve unmapped data ---
    cost = src.get("cost", 0)
    if cost != 0:
        unused["cost"] = cost
    rent = src.get("rent", 0)
    if rent != 0:
        unused["rent"] = rent
    weight = src.get("weight", 0)
    if weight != 0:
        unused["weight"] = weight
    if skipped_effects:
        unused["effects"] = skipped_effects
    if type_note not in ("WEAPON", "ARMOR", "LIGHT", "CONTAINER", "KEY",
                          "TREASURE", "OTHER", "TRASH"):
        unused["type"] = type_note
        unused["values"] = values

    # For containers, move closure from unused into the spec proper.
    closure_data = unused.pop("closure", None)
    if closure_data:
        obj["closure"] = closure_data

    result = {
        "version": 1,
        "id": oid,
        "spec": obj,
    }

    if unused:
        result["circlemud_unused"] = unused

    return result


# ---------------------------------------------------------------------------
# Zone placement: object spawns in rooms
# ---------------------------------------------------------------------------

def process_zone_placements(zones, v2z, all_zone_data):
    """Process zone reset commands to build room_id -> [ObjectSpawn, ...]."""
    room_spawns = {}  # room_id -> [ObjectSpawn, ...]

    for zone_num, zone_data in all_zone_data.items():
        for zone in zone_data:
            for obj_entry in zone.get("objects", []):
                ovnum = obj_entry["id"]
                rvnum = obj_entry["room"]
                oid = obj_id(ovnum, v2z, zones)
                rid = room_id(rvnum, v2z, zones)

                spawn = {"object_id": oid}

                # Nested contents (P commands)
                contents = []
                for child in obj_entry.get("contents", []):
                    cvnum = child["id"]
                    cid = obj_id(cvnum, v2z, zones)
                    contents.append({"object_id": cid})
                if contents:
                    spawn["contents"] = contents

                if rid not in room_spawns:
                    room_spawns[rid] = []
                room_spawns[rid].append(spawn)

    return room_spawns


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    objects_dir = os.path.join(OUTPUT_DIR, "objects")
    rooms_dir = os.path.join(OUTPUT_DIR, "rooms")

    print("Fetching zone metadata...", flush=True)
    zones = build_zone_info()
    v2z = build_vnum_to_zone(zones)

    # Fetch all zone placement data
    print("Fetching zone placement data...", flush=True)
    all_zone_data = {}
    for zn in ZONE_NUMS:
        all_zone_data[zn] = fetch_json(f"zon/{zn}.json")

    # Fetch all objects and register vnums into v2z
    all_objs = {}
    for zn in ZONE_NUMS:
        print(f"Fetching objects for zone {zn}...", end=" ", flush=True)
        try:
            objs = fetch_json(f"obj/{zn}.json")
        except Exception as e:
            print(f"ERROR: {e}")
            continue
        all_objs[zn] = objs
        zone_slug = zones[zn]["slug"]
        for obj in objs:
            v2z[obj["id"]] = zone_slug
        print(f"{len(objs)} objects")

    # Remove old stub files from flat objects dir
    stubs_removed = 0
    for fname in os.listdir(objects_dir):
        if fname.endswith(".json"):
            os.remove(os.path.join(objects_dir, fname))
            stubs_removed += 1
    if stubs_removed:
        print(f"Removed {stubs_removed} old stub files")

    # Process zone placements
    room_spawns = process_zone_placements(zones, v2z, all_zone_data)

    # Collect all known object vnums for key reference validation
    known_obj_vnums = set()
    for objs in all_objs.values():
        for obj in objs:
            known_obj_vnums.add(obj["id"])

    # Write object assets
    total_objs = 0
    for zn, objs in all_objs.items():
        zone_slug = zones[zn]["slug"]
        zone_obj_dir = os.path.join(objects_dir, zone_slug)
        os.makedirs(zone_obj_dir, exist_ok=True)

        for src_obj in objs:
            out = convert_object(src_obj, zone_slug, v2z, zones, known_obj_vnums)
            filename = f"{out['id']}.json"
            filepath = os.path.join(zone_obj_dir, filename)
            with open(filepath, "w") as f:
                json.dump(out, f, indent=4)
                f.write("\n")
            total_objs += 1

    # Update room files with object_spawns
    rooms_updated = 0
    for rid, spawns in room_spawns.items():
        parts = rid.rsplit("-", 1)
        if len(parts) != 2:
            continue
        vnum = parts[1]
        zone_slug = rid[:-(len(vnum) + 1)]
        room_file = os.path.join(rooms_dir, zone_slug, f"{rid}.json")
        if not os.path.exists(room_file):
            print(f"  WARN: room file not found for {rid}")
            continue

        with open(room_file) as f:
            room_data = json.load(f)

        room_data["spec"]["object_spawns"] = spawns

        with open(room_file, "w") as f:
            json.dump(room_data, f, indent=4)
            f.write("\n")
        rooms_updated += 1

    print(f"\nTotal: {total_objs} objects, {rooms_updated} rooms updated with object_spawns")


if __name__ == "__main__":
    main()

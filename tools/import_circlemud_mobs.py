#!/usr/bin/env python3
"""Convert CircleMUD parsed mob JSON into go-mud Mobile assets.

Reads mob definitions from _output/mob/ and zone placement data from _output/zon/.
Writes:
  - Mobile assets to circlemud/mobiles/<zone-slug>/
  - Updates room files in circlemud/rooms/ with mobile_spawns
  - Stubs missing objects referenced by mob inventory/equipment

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

# CircleMUD equipment slot location -> our slot name
EQUIP_SLOTS = {
    "LIGHT": "light",
    "RING_R": "finger",
    "RING_L": "finger",
    "NECK_1": "neck",
    "NECK_2": "neck",
    "BODY": "body",
    "HEAD": "head",
    "LEGS": "legs",
    "FEET": "feet",
    "HANDS": "hands",
    "ARMS": "arms",
    "SHIELD": "shield",
    "ABOUT": "about",
    "WAIST": "waist",
    "WRIST_R": "wrist",
    "WRIST_L": "wrist",
    "WIELD": "wield",
    "HOLD": "hold",
}

# CircleMUD mob flags -> our mobile flags (behavior properties)
MOB_FLAGS = {
    "SENTINEL": "sentinel",
    "AGGRESSIVE": "aggressive",
    "WIMPY": "wimpy",
    "HELPER": "helper",
    "STAY_ZONE": "stay_zone",
    "SCAVENGER": "scavenger",
    "MEMORY": "memory",
    "AWARE": "aware",
}

# CircleMUD affection flags -> our perk grants
AFFECT_GRANTS = {
    "INVISIBLE": "invisible",
    "DETECT_INVIS": "detect_invis",
    "SENSE_LIFE": "sense_life",
    "WATERWALK": "waterwalk",
    "INFRAVISION": "infravision",
    "SNEAK": "sneak",
    "HIDE": "hide",
    "PROTECT_EVIL": "protect_evil",
    "PROTECT_GOOD": "protect_good",
    "NOTRACK": "notrack",
    "NOCHARM": "nocharm",
    "NOSUMMON": "nosummon",
    "NOSLEEP": "nosleep",
    "NOBASH": "nobash",
    "NOBLIND": "noblind",
}


def slugify(name):
    return name.lower().replace("'", "").replace(",", "").replace(".", "").replace("  ", " ").replace(" ", "-").strip("-")


def collapse_newlines(s):
    return re.sub(r"\s+", " ", s).strip()


def fetch_json(path):
    url = f"{BASE_URL}/{path}"
    with urllib.request.urlopen(url) as resp:
        return json.loads(resp.read())


# ---------------------------------------------------------------------------
# Zone metadata (shared with room importer)
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


def mob_id(vnum, v2z, zones):
    return vnum_to_id(vnum, v2z, zones)


def obj_id(vnum, v2z, zones):
    return vnum_to_id(vnum, v2z, zones)


def room_id(vnum, v2z, zones):
    return vnum_to_id(vnum, v2z, zones)


# ---------------------------------------------------------------------------
# Mob conversion
# ---------------------------------------------------------------------------

def convert_mob(src, zone_slug, v2z, zones):
    perks = []
    flags = []

    # Behavior flags
    for f in src.get("flags", []):
        note = f.get("note", "")
        if note in MOB_FLAGS:
            flags.append(MOB_FLAGS[note])

    # Affection grants
    for a in src.get("affects", []):
        note = a.get("note", "")
        if note == "SANCTUARY":
            perks.append({"type": "modifier", "key": "core.defense.all.absorb.pct", "value": 50})
        elif note in AFFECT_GRANTS:
            perks.append({"type": "grant", "key": AFFECT_GRANTS[note]})

    # Armor class
    ac = src.get("armor_class")
    if ac is not None and ac != 0:
        perks.append({"type": "modifier", "key": "core.combat.ac.flat", "value": ac})

    # Bare hand damage
    bhd = src.get("bare_hand_damage", {})
    if bhd.get("dice") and bhd.get("sides"):
        dice_expr = f"{bhd['dice']}d{bhd['sides']}"
        if bhd.get("bonus", 0) > 0:
            dice_expr += f"+{bhd['bonus']}"
        perks.append({"type": "grant", "key": "attack", "arg": dice_expr})
        perks.append({"type": "grant", "key": "auto_use", "arg": "attack:1"})

    # Max HP (dice*sides + bonus)
    mhp = src.get("max_hit_points", {})
    if mhp:
        hp = mhp.get("dice", 1) * mhp.get("sides", 1) + mhp.get("bonus", 0)
        if hp > 0:
            perks.append({"type": "modifier", "key": "core.resource.hp.max", "value": hp})

    # THAC0 -> attack bonus (CircleMUD THAC0: lower = better, 20 = base)
    thac0 = src.get("thac0")
    if thac0 is not None and thac0 != 20:
        attack_bonus = 20 - thac0
        if attack_bonus != 0:
            perks.append({"type": "modifier", "key": "core.combat.attack.flat", "value": attack_bonus})

    vnum = src["id"]
    mid = mob_id(vnum, v2z, zones)

    mobile = {
        "aliases": src.get("aliases", []),
        "short_desc": src.get("short_desc", "").strip(),
        "long_desc": src.get("long_desc", "").strip(),
        "detailed_desc": collapse_newlines(src.get("detail_desc", "")),
        "level": src.get("level", 1),
    }

    if perks:
        mobile["perks"] = perks
    if flags:
        mobile["flags"] = flags

    exp = src.get("xp", 0)
    if exp > 0:
        mobile["exp_reward"] = exp

    return {
        "version": 1,
        "id": mid,
        "spec": mobile,
    }


# ---------------------------------------------------------------------------
# Zone placement: mob spawns, inventory, equipment
# ---------------------------------------------------------------------------

def process_zone_placements(zones, v2z, all_zone_data):
    """Process zone reset commands to build:
    - room_spawns: room_id -> [mob_id, ...]
    - mob_inventory: mob_id -> [ObjectSpawn, ...]
    - mob_equipment: mob_id -> [EquipmentSpawn, ...]
    - referenced_obj_ids: set of object IDs that need to exist
    """
    room_spawns = {}      # room_id -> [mob_id, ...]
    mob_inventory = {}    # mob_id -> [{"object_id": ...}, ...]
    mob_equipment = {}    # mob_id -> [{"slot": ..., "object_id": ...}, ...]
    referenced_objs = set()

    for zone_num, zone_data in all_zone_data.items():
        for zone in zone_data:
            for mob_entry in zone.get("mobs", []):
                mvnum = mob_entry["mob"]
                rvnum = mob_entry["room"]
                mid = mob_id(mvnum, v2z, zones)
                rid = room_id(rvnum, v2z, zones)

                # Room spawn
                if rid not in room_spawns:
                    room_spawns[rid] = []
                room_spawns[rid].append(mid)

                # Inventory (G commands)
                for inv_item in mob_entry.get("inventory", []):
                    oid = obj_id(inv_item["id"], v2z, zones)
                    referenced_objs.add(oid)
                    if mid not in mob_inventory:
                        mob_inventory[mid] = []
                    mob_inventory[mid].append({"object_id": oid})

                # Equipment (E commands)
                for eq_item in mob_entry.get("equipped", []):
                    slot_note = eq_item.get("note", "")
                    slot = EQUIP_SLOTS.get(slot_note)
                    if slot is None:
                        continue
                    oid = obj_id(eq_item["id"], v2z, zones)
                    referenced_objs.add(oid)
                    if mid not in mob_equipment:
                        mob_equipment[mid] = []
                    mob_equipment[mid].append({"slot": slot, "object_id": oid})

    return room_spawns, mob_inventory, mob_equipment, referenced_objs


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    mobiles_dir = os.path.join(OUTPUT_DIR, "mobiles")
    rooms_dir = os.path.join(OUTPUT_DIR, "rooms")
    objects_dir = os.path.join(OUTPUT_DIR, "objects")

    print("Fetching zone metadata...", flush=True)
    zones = build_zone_info()
    v2z = build_vnum_to_zone(zones)

    # Fetch all zone placement data
    print("Fetching zone placement data...", flush=True)
    all_zone_data = {}
    for zn in ZONE_NUMS:
        all_zone_data[zn] = fetch_json(f"zon/{zn}.json")

    # Register mob vnums into v2z (mobs can be outside declared room range)
    all_mobs = {}
    for zn in ZONE_NUMS:
        print(f"Fetching mobs for zone {zn}...", end=" ", flush=True)
        try:
            mobs = fetch_json(f"mob/{zn}.json")
        except Exception as e:
            print(f"ERROR: {e}")
            continue
        all_mobs[zn] = mobs
        zone_slug = zones[zn]["slug"]
        for mob in mobs:
            v2z[mob["id"]] = zone_slug
        print(f"{len(mobs)} mobs")

    # Process zone placements
    room_spawns, mob_inventory, mob_equipment, referenced_objs = process_zone_placements(
        zones, v2z, all_zone_data
    )

    # Write mob assets
    total_mobs = 0
    for zn, mobs in all_mobs.items():
        zone_slug = zones[zn]["slug"]
        zone_mob_dir = os.path.join(mobiles_dir, zone_slug)
        os.makedirs(zone_mob_dir, exist_ok=True)

        for src_mob in mobs:
            out = convert_mob(src_mob, zone_slug, v2z, zones)
            mid = out["id"]

            # Add inventory from zone data
            if mid in mob_inventory:
                out["spec"]["inventory"] = mob_inventory[mid]
            if mid in mob_equipment:
                out["spec"]["equipment"] = mob_equipment[mid]

            filename = f"{mid}.json"
            filepath = os.path.join(zone_mob_dir, filename)
            with open(filepath, "w") as f:
                json.dump(out, f, indent=4)
                f.write("\n")
            total_mobs += 1

    # Update room files with mobile_spawns
    rooms_updated = 0
    for rid, mob_ids in room_spawns.items():
        # Find the room file
        parts = rid.rsplit("-", 1)
        if len(parts) != 2:
            continue
        vnum = parts[1]
        # Determine zone slug from room id
        zone_slug = rid[:-(len(vnum) + 1)]
        room_file = os.path.join(rooms_dir, zone_slug, f"{rid}.json")
        if not os.path.exists(room_file):
            print(f"  WARN: room file not found for {rid}")
            continue

        with open(room_file) as f:
            room_data = json.load(f)

        room_data["spec"]["mobile_spawns"] = mob_ids

        with open(room_file, "w") as f:
            json.dump(room_data, f, indent=4)
            f.write("\n")
        rooms_updated += 1

    # Stub missing objects
    existing_objs = set()
    for root, dirs, files in os.walk(objects_dir):
        for fname in files:
            if fname.endswith(".json"):
                with open(os.path.join(root, fname)) as f:
                    existing_objs.add(json.load(f)["id"])

    new_stubs = 0
    for oid in sorted(referenced_objs):
        if oid in existing_objs:
            continue
        stub = {
            "version": 1,
            "id": oid,
            "spec": {
                "aliases": [oid.rsplit("-", 1)[-1]],
                "short_desc": f"a {oid.rsplit('-', 1)[-1]}",
            }
        }
        filepath = os.path.join(objects_dir, f"{oid}.json")
        with open(filepath, "w") as f:
            json.dump(stub, f, indent=4)
            f.write("\n")
        new_stubs += 1

    print(f"\nTotal: {total_mobs} mobs, {rooms_updated} rooms updated, {new_stubs} new object stubs")


if __name__ == "__main__":
    main()

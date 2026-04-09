#!/usr/bin/env python3
"""Convert CircleMUD parsed room JSON into go-mud room asset JSON.

Reads from https://github.com/isms/circlemud-world-parser/_output/
and writes one go-mud JSON file per room into circlemud/rooms/.
Also writes zone asset files into circlemud/zones/.

Rooms are grouped by their zone_number. Exits that cross zone boundaries
include a zone_id field.
"""

import json
import os
import sys
import urllib.request

BASE_URL = "https://raw.githubusercontent.com/isms/circlemud-world-parser/master/_output"
OUTPUT_DIR = os.path.join(os.path.dirname(__file__), "..", "circlemud")

DIRECTIONS = {0: "north", 1: "east", 2: "south", 3: "west", 4: "up", 5: "down"}

ROOM_FLAGS = {
    "DARK": {"type": "perk", "perk": {"type": "grant", "key": "dark"}},
    "DEATH": {"type": "flag", "flag": "death"},
    "NOMOB": {"type": "flag", "flag": "nomob"},
    "PEACEFUL": {"type": "perk", "perk": {"type": "grant", "key": "peaceful"}},
    "NOMAGIC": {"type": "perk", "perk": {"type": "grant", "key": "nomagic"}},
    "TUNNEL": {"type": "flag", "flag": "single_occupant"},
}

# CircleMUD reset_mode: 0=never, 1=only when empty, 2=always on lifespan
RESET_MODES = {0: "never", 1: "empty", 2: "lifespan"}


def collapse_newlines(s):
    """Replace newlines with spaces and collapse multiple spaces."""
    import re
    return re.sub(r"\s+", " ", s).strip()

WLD_FILES = [
    0, 9, 12, 15, 25, 30, 31, 33, 35, 36, 40, 50, 51, 52, 53, 54,
    60, 61, 62, 63, 64, 65, 70, 71, 72, 79, 120, 150, 186,
]


def slugify(name):
    """Convert a zone name to a slug for IDs."""
    return name.lower().replace("'", "").replace(",", "").replace(".", "").replace("  ", " ").replace(" ", "-").strip("-")


# --- Build zone lookup ---

def fetch_json(path):
    url = f"{BASE_URL}/{path}"
    with urllib.request.urlopen(url) as resp:
        return json.loads(resp.read())


def build_zone_info():
    """Fetch zone metadata and build lookup from zone_number -> zone info."""
    zones = {}
    for zn in WLD_FILES:
        data = fetch_json(f"zon/{zn}.json")
        for z in data:
            slug = slugify(z["name"])
            zones[z["id"]] = {
                "id": z["id"],
                "name": z["name"],
                "slug": slug,
                "lifespan": z.get("lifespan", 0),
                "reset_mode": z.get("reset_mode", 0),
                "bottom_room": z.get("bottom_room", 0),
                "top_room": z.get("top_room", 0),
            }
    return zones


def build_room_to_zone(zones):
    """Build a mapping from room vnum -> zone slug."""
    r2z = {}
    for z in zones.values():
        for vnum in range(z["bottom_room"], z["top_room"] + 1):
            r2z[vnum] = z["slug"]
    return r2z


# --- Room conversion ---

def room_id(zone_slug, vnum):
    return f"{zone_slug}-{vnum}"


def vnum_to_id(vnum, vnum_to_zone, zones):
    """Convert any CircleMUD vnum to a zone-prefixed ID."""
    zone_slug = vnum_to_zone.get(vnum)
    if zone_slug is None:
        # Find the zone whose range is closest (object vnums often sit just
        # outside the declared room range but belong to the same zone).
        best = None
        for z in zones.values():
            if z["bottom_room"] <= vnum <= z["top_room"]:
                best = z["slug"]
                break
            # Check if vnum is just past top_room (same zone prefix)
            if vnum > z["top_room"] and vnum < z["top_room"] + 100:
                if best is None:
                    best = z["slug"]
        zone_slug = best or "circlemud"
    return f"{zone_slug}-{vnum}"


def convert_exit(ex, source_zone_slug, vnum_to_zone, zones, known_obj_vnums):
    dest_vnum = ex["room_linked"]
    if dest_vnum < 0:
        return None

    dest_zone_slug = vnum_to_zone.get(dest_vnum, source_zone_slug)

    result = {
        "room_id": room_id(dest_zone_slug, dest_vnum),
    }

    if dest_zone_slug != source_zone_slug:
        result["zone_id"] = dest_zone_slug

    if ex.get("desc"):
        result["description"] = collapse_newlines(ex["desc"])

    door_flag = ex.get("door_flag", {}).get("value", 0)
    if door_flag > 0:
        keywords = ex.get("keywords", [])
        name = keywords[0] if keywords else "door"
        closure = {
            "name": name,
            "closed": True,
        }
        key_num = ex.get("key_number", -1)
        if key_num > 0 and key_num in known_obj_vnums:
            lock = {
                "key_id": vnum_to_id(key_num, vnum_to_zone, zones),
                "locked": True,
            }
            if door_flag == 2:
                lock["pickproof"] = True
            closure["lock"] = lock
        result["closure"] = closure

    return result


def convert_room(src, zone_slug, vnum_to_zone, zones, known_obj_vnums):
    flags = []
    perks = []

    skipped_flags = []
    for f in src.get("flags", []):
        note = f.get("note", "")
        mapping = ROOM_FLAGS.get(note)
        if mapping is None:
            if note:
                skipped_flags.append(note)
            continue
        if mapping["type"] == "flag":
            flags.append(mapping["flag"])
        elif mapping["type"] == "perk":
            perks.append(mapping["perk"])

    exits = {}
    for ex in src.get("exits", []):
        direction = DIRECTIONS.get(ex.get("dir"))
        if direction is None:
            continue
        converted = convert_exit(ex, zone_slug, vnum_to_zone, zones, known_obj_vnums)
        if converted is None:
            continue
        exits[direction] = converted

    extra_descs = []
    for ed in src.get("extra_descs", []):
        keywords = ed.get("keywords", [])
        desc = collapse_newlines(ed.get("desc", ""))
        if keywords and desc:
            extra_descs.append({"keywords": keywords, "description": desc})

    vnum = src["id"]
    room = {
        "name": src.get("name", "").strip(),
        "description": collapse_newlines(src.get("desc", "")),
        "zone_id": zone_slug,
        "exits": exits,
    }

    if perks:
        room["perks"] = perks
    if flags:
        room["flags"] = flags
    if extra_descs:
        room["extra_descs"] = extra_descs

    # --- circlemud_unused: preserve unmapped data ---
    unused = {}
    sector = src.get("sector_type", {}).get("note", "")
    if sector:
        unused["sector_type"] = sector
    if skipped_flags:
        unused["flags"] = skipped_flags

    result = {
        "version": 1,
        "id": room_id(zone_slug, vnum),
        "spec": room,
    }

    if unused:
        result["circlemud_unused"] = unused

    return result


# --- Zone asset conversion ---

def convert_zone(zone_info):
    reset_mode = RESET_MODES.get(zone_info["reset_mode"], "lifespan")
    zone = {
        "name": zone_info["name"],
        "reset_mode": reset_mode,
    }
    if reset_mode != "never" and zone_info["lifespan"] > 0:
        zone["lifespan"] = f"{zone_info['lifespan']}m"

    return {
        "version": 1,
        "id": zone_info["slug"],
        "spec": zone,
    }


def main():
    rooms_dir = os.path.join(OUTPUT_DIR, "rooms")
    zones_dir = os.path.join(OUTPUT_DIR, "zones")
    os.makedirs(rooms_dir, exist_ok=True)
    os.makedirs(zones_dir, exist_ok=True)

    print("Fetching zone metadata...", flush=True)
    zones = build_zone_info()
    vnum_to_zone = build_room_to_zone(zones)

    # Write zone assets
    for z in zones.values():
        out = convert_zone(z)
        filepath = os.path.join(zones_dir, f"{z['slug']}.json")
        with open(filepath, "w") as f:
            json.dump(out, f, indent=4)
            f.write("\n")
    print(f"Wrote {len(zones)} zone files")

    # Fetch all rooms first so we can register vnums that fall outside declared ranges.
    all_rooms = {}  # zone_num -> list of source rooms
    for zone_num in WLD_FILES:
        print(f"Fetching rooms for zone {zone_num}...", end=" ", flush=True)
        try:
            rooms = fetch_json(f"wld/{zone_num}.json")
        except Exception as e:
            print(f"ERROR: {e}")
            continue
        all_rooms[zone_num] = rooms
        zone_slug = zones[zone_num]["slug"]
        for src_room in rooms:
            vnum_to_zone[src_room["id"]] = zone_slug
        print(f"{len(rooms)} rooms")

    # Fetch known object vnums for key reference validation
    print("Fetching object vnums for key validation...", flush=True)
    known_obj_vnums = set()
    for zn in WLD_FILES:
        try:
            objs = fetch_json(f"obj/{zn}.json")
            for o in objs:
                known_obj_vnums.add(o["id"])
        except Exception:
            pass
    print(f"Found {len(known_obj_vnums)} known objects")

    # Write room assets
    total = 0
    for zone_num, rooms in all_rooms.items():
        zone_slug = zones[zone_num]["slug"]
        zone_rooms_dir = os.path.join(rooms_dir, zone_slug)
        os.makedirs(zone_rooms_dir, exist_ok=True)
        for src_room in rooms:
            out = convert_room(src_room, zone_slug, vnum_to_zone, zones, known_obj_vnums)
            vnum = src_room["id"]
            filename = f"{zone_slug}-{vnum}.json"
            filepath = os.path.join(zone_rooms_dir, filename)
            with open(filepath, "w") as f:
                json.dump(out, f, indent=4)
                f.write("\n")
            total += 1

    print(f"\nTotal: {total} rooms, {len(zones)} zones written to {OUTPUT_DIR}")


if __name__ == "__main__":
    main()

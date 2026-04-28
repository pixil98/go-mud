#!/usr/bin/env python3
"""Convert CircleMUD source world files into go-mud asset JSON.

Parses the original CircleMUD .wld, .mob, .obj, and .zon files directly
(not the third-party parsed JSON) and writes go-mud assets to circlemud/.

Fetches raw source files from the CircleMUD GitHub repo.
"""

import json
import os
import re
import sys
import urllib.request

BASE_URL = "https://raw.githubusercontent.com/Yuffster/CircleMUD/master/lib/world"
OUTPUT_DIR = os.path.join(os.path.dirname(__file__), "..", "circlemud")

# ---------------------------------------------------------------------------
# Directions
# ---------------------------------------------------------------------------

DIRECTIONS = {0: "north", 1: "east", 2: "south", 3: "west", 4: "up", 5: "down"}

# ---------------------------------------------------------------------------
# Room constants
# ---------------------------------------------------------------------------

ROOM_FLAG_BITS = {
    0: "DARK",
    1: "DEATH",
    2: "NOMOB",
    3: "INDOORS",
    4: "PEACEFUL",
    5: "SOUNDPROOF",
    6: "NOTRACK",
    7: "NOMAGIC",
    8: "TUNNEL",
    9: "PRIVATE",
    10: "GODROOM",
}

# Each entry is the perk body emitted (without "type": "grant", which is added
# by the conversion). Room properties are keyed "room_<name>" so the audience
# is visible in the identifier; actor-target perks (none here yet) would stay
# unprefixed.
ROOM_FLAGS = {
    "DARK": {"key": "room_dark"},
    "DEATH": {"key": "room_death"},
    "NOMOB": {"key": "room_nomob"},
    "PEACEFUL": {"key": "room_peaceful"},
    "NOMAGIC": {"key": "room_nomagic"},
    "TUNNEL": {"key": "room_single_occupant"},
}

# Sector types that imply a room perk (e.g. deep water requires waterwalk).
SECTOR_FLAGS = {
    "WATER_NOSWIM": {"key": "room_water"},
}

SECTOR_TYPES = {
    0: "INSIDE", 1: "CITY", 2: "FIELD", 3: "FOREST", 4: "HILLS",
    5: "MOUNTAIN", 6: "WATER_SWIM", 7: "WATER_NOSWIM", 8: "UNDERWATER",
    9: "FLYING",
}

# ---------------------------------------------------------------------------
# Mobile constants
# ---------------------------------------------------------------------------

MOB_ACTION_BITS = {
    0: "SPEC",
    1: "SENTINEL",
    2: "SCAVENGER",
    3: "ISNPC",
    4: "AWARE",
    5: "AGGRESSIVE",
    6: "STAY_ZONE",
    7: "WIMPY",
    8: "AGGR_EVIL",
    9: "AGGR_GOOD",
    10: "AGGR_NEUTRAL",
    11: "MEMORY",
    12: "HELPER",
    13: "NOCHARM",
    14: "NOSUMMON",
    15: "NOSLEEP",
    16: "NOBASH",
    17: "NOBLIND",
}

MOB_AFFECT_BITS = {
    0: "BLIND",
    1: "INVISIBLE",
    2: "DETECT_ALIGN",
    3: "DETECT_INVIS",
    4: "DETECT_MAGIC",
    5: "SENSE_LIFE",
    6: "WATERWALK",
    7: "SANCTUARY",
    8: "GROUP",
    9: "CURSE",
    10: "INFRAVISION",
    11: "POISON",
    12: "PROTECT_EVIL",
    13: "PROTECT_GOOD",
    14: "SLEEP",
    15: "NOTRACK",
    18: "SNEAK",
    19: "HIDE",
}

# Action flags we map to our mobile flags.
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

# Affect flags we map to perk grants. Each value is the grant body (key + optional arg)
# without the "type" field; the importer adds "type": "grant" before emitting.
AFFECT_GRANTS = {
    "INVISIBLE": {"key": "invisible"},
    "DETECT_INVIS": {"key": "detect_invis"},
    "SENSE_LIFE": {"key": "sense_life"},
    "WATERWALK": {"key": "ignore_restriction", "arg": "room_water"},
    "INFRAVISION": {"key": "ignore_restriction", "arg": "room_dark"},
    "SNEAK": {"key": "sneak"},
    "HIDE": {"key": "hide"},
    "PROTECT_EVIL": {"key": "protect_evil"},
    "PROTECT_GOOD": {"key": "protect_good"},
    "NOTRACK": {"key": "notrack"},
    "NOCHARM": {"key": "nocharm"},
    "NOSUMMON": {"key": "nosummon"},
    "NOSLEEP": {"key": "nosleep"},
    "NOBASH": {"key": "nobash"},
    "NOBLIND": {"key": "noblind"},
}


# E-spec mob stats we care about are handled inline.

# ---------------------------------------------------------------------------
# Object constants
# ---------------------------------------------------------------------------

OBJ_TYPE_MAP = {
    1: "LIGHT", 2: "SCROLL", 3: "WAND", 4: "STAFF", 5: "WEAPON",
    6: "FIREWEAPON", 7: "MISSILE", 8: "TREASURE", 9: "ARMOR", 10: "POTION",
    11: "WORN", 12: "OTHER", 13: "TRASH", 14: "TRAP", 15: "CONTAINER",
    16: "NOTE", 17: "DRINKCON", 18: "KEY", 19: "FOOD", 20: "MONEY",
    21: "PEN", 22: "BOAT", 23: "FOUNTAIN",
}

OBJ_EFFECT_BITS = {
    0: "GLOW",
    1: "HUM",
    2: "NORENT",
    3: "NODONATE",
    4: "NOINVIS",
    5: "INVISIBLE",
    6: "MAGIC",
    7: "NODROP",
    8: "BLESS",
    9: "ANTI_GOOD",
    10: "ANTI_EVIL",
    11: "ANTI_NEUTRAL",
    12: "ANTI_MAGIC_USER",
    13: "ANTI_CLERIC",
    14: "ANTI_THIEF",
    15: "ANTI_WARRIOR",
    16: "NOSELL",
}

OBJ_WEAR_BITS = {
    0: "WEAR_TAKE",
    1: "WEAR_FINGER",
    2: "WEAR_NECK",
    3: "WEAR_BODY",
    4: "WEAR_HEAD",
    5: "WEAR_LEGS",
    6: "WEAR_FEET",
    7: "WEAR_HANDS",
    8: "WEAR_ARMS",
    9: "WEAR_SHIELD",
    10: "WEAR_ABOUT",
    11: "WEAR_WAIST",
    12: "WEAR_WRIST",
    13: "WEAR_WIELD",
    14: "WEAR_HOLD",
}

EFFECT_FLAGS = {
    "INVISIBLE": "invisible",
    "NODROP": "no_drop",
    "NOSELL": "no_sell",
}

EFFECT_SKIP = {
    "NORENT", "NODONATE", "NOINVIS", "MAGIC", "BLESS", "GLOW", "HUM",
    "ANTI_GOOD", "ANTI_EVIL", "ANTI_NEUTRAL",
    "ANTI_MAGIC_USER", "ANTI_CLERIC", "ANTI_THIEF", "ANTI_WARRIOR",
}

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

AFFECT_LOCATION_MAP = {
    1: "STR", 2: "DEX", 3: "INT", 4: "WIS", 5: "CON", 6: "CHA",
    9: "AGE", 12: "MANA", 13: "HIT", 14: "MOVE",
    17: "AC", 18: "HITROLL", 19: "DAMROLL",
}

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

WEAPON_TYPES = {
    0: "hit", 1: "sting", 2: "whip", 3: "slash", 4: "bite",
    5: "bludgeon", 6: "crush", 7: "pound", 8: "claw", 9: "maul",
    10: "thrash", 11: "pierce", 12: "blast", 13: "punch", 14: "stab",
}

CONTAINER_CLOSEABLE = 1
CONTAINER_PICKPROOF = 2
CONTAINER_CLOSED = 4
CONTAINER_LOCKED = 8

RESET_MODES = {0: "never", 1: "empty", 2: "lifespan"}

# Equipment position number -> our slot name.
EQUIP_POSITION_MAP = {
    0: "light",
    1: "finger",   # right finger
    2: "finger",   # left finger
    3: "neck",     # neck 1
    4: "neck",     # neck 2
    5: "body",
    6: "head",
    7: "legs",
    8: "feet",
    9: "hands",
    10: "arms",
    11: "shield",
    12: "about",
    13: "waist",
    14: "wrist",   # right wrist
    15: "wrist",   # left wrist
    16: "wield",
    17: "hold",
}

# Sex number -> string for circlemud_unused.
SEX_MAP = {0: "NEUTRAL", 1: "MALE", 2: "FEMALE"}

# Zone slug overrides (slugify doesn't always produce the desired result).
ZONE_SLUG_OVERRIDES = {
    "limbo-internal": "limbo",
}

# Zone slug -> creator credit (from circlemud.org/world.html).
ZONE_CREATORS = {
    "limbo": "DikuMud",
    "river-island-of-minos": "Mahatma of HexOynx",
    "god-simplex": "CircleMUD, modified by Taz of Tazmania",
    "the-straight-path": "Steppin of ChicagoMUD",
    "the-high-tower-of-magic": "Skylar of SillyMUD",
    "northern-midgaard-main-city": "DikuMud",
    "southern-part-of-midgaard": "DikuMud",
    "the-three-of-swords": "C.A.W.",
    "midennir": "Copper II, modified by VampLestat of MercMUD",
    "the-chessboard-of-midgaard": "Exxon of SillyMUD",
    "mines-of-moria": "Redferne of DikuMud",
    "the-great-eastern-desert": "Rorschach",
    "the-city-of-thalos": "Rorschach",
    "new-thalos": "Duke of SillyMUD",
    "the-great-pyramid": "Andersen of HexOynx",
    "drow-city": "Rorschach",
    "haon-dor-light-forest": "Quifael of DikuMud, modified by Derkhil of CircleMUD",
    "haon-dor-dark-forest": "Quifael of DikuMud, modified by Derkhil of CircleMUD",
    "the-dwarven-kingdom": "Depeche of DikuMud",
    "the-orc-enclave": "C.A.W.",
    "rands-tower": "C.A.W.",
    "arachnos": "Mahatma of HexOynx",
    "the-sewer-first-level": "Redferne of DikuMud",
    "the-second-sewer": "Redferne of DikuMud",
    "the-sewer-maze": "Redferne of DikuMud",
    "the-tunnels-in-the-sewer": "Redferne of DikuMud",
    "redfernes-residence": "Redferne of DikuMud, modified by Cyron of VieMud",
    "rome": "Onivel of JediMUD",
    "king-welmars-castle": "Pjotr and Sapowox of CircleMUD",
    "newbie-zone": "Maynard of StrangeMUD",
}


# ---------------------------------------------------------------------------
# Shared utilities
# ---------------------------------------------------------------------------

def slugify(name):
    s = name.lower().replace("'", "").replace(",", "").replace(".", "")
    s = s.replace("  ", " ").replace(" ", "-").strip("-")
    return re.sub(r"-+", "-", s)


def collapse_newlines(s):
    return re.sub(r"\s+", " ", s).strip()


def fetch_text(path):
    url = f"{BASE_URL}/{path}"
    with urllib.request.urlopen(url) as resp:
        return resp.read().decode("utf-8", errors="replace")


def decode_bitvector(token):
    """Decode a CircleMUD bitvector token (numeric or letter-encoded)."""
    token = token.strip()
    try:
        return int(token)
    except ValueError:
        pass
    value = 0
    for ch in token:
        if "a" <= ch <= "z":
            value |= 1 << (ord(ch) - ord("a"))
        elif "A" <= ch <= "Z":
            value |= 1 << (ord(ch) - ord("A") + 26)
    return value


def bitvector_to_names(value, bit_table):
    """Convert a decoded bitvector int to a list of flag name strings."""
    names = []
    for bit, name in sorted(bit_table.items()):
        if value & (1 << bit):
            names.append(name)
    return names


def parse_dice(s):
    """Parse 'XdY+Z' into {dice, sides, bonus}."""
    m = re.match(r"(\d+)d(\d+)([+-]\d+)?", s.strip())
    if not m:
        return {"dice": 0, "sides": 0, "bonus": 0}
    return {
        "dice": int(m.group(1)),
        "sides": int(m.group(2)),
        "bonus": int(m.group(3)) if m.group(3) else 0,
    }


# ---------------------------------------------------------------------------
# Raw source file parsers
# ---------------------------------------------------------------------------

def fetch_index(subdir):
    """Fetch and parse an index file, returning list of file numbers."""
    text = fetch_text(f"{subdir}/index")
    nums = []
    for line in text.strip().splitlines():
        line = line.strip()
        if line == "$":
            break
        # Lines like "0.wld" or "25.zon" — extract the number.
        m = re.match(r"(\d+)\.", line)
        if m:
            nums.append(int(m.group(1)))
    return nums


class LineReader:
    """Simple line-based reader with tilde-terminated field support."""

    def __init__(self, text):
        self.lines = text.splitlines()
        self.pos = 0

    def eof(self):
        return self.pos >= len(self.lines)

    def peek(self):
        if self.eof():
            return None
        return self.lines[self.pos]

    def next(self):
        line = self.lines[self.pos]
        self.pos += 1
        return line

    def read_tilde_string(self):
        """Read a tilde-terminated string field.

        The tilde may appear at the end of the first line (single-line field)
        or on a subsequent line (multi-line field). Everything up to the first
        ~ is returned.
        """
        parts = []
        while not self.eof():
            line = self.next()
            idx = line.find("~")
            if idx >= 0:
                parts.append(line[:idx])
                break
            parts.append(line)
        return "\n".join(parts)


def parse_zon(text):
    """Parse a .zon file, returning a zone dict."""
    reader = LineReader(text)

    # Skip to #zone_num
    while not reader.eof():
        line = reader.peek()
        if line.startswith("#"):
            break
        reader.next()

    zone_num = int(reader.next().lstrip("#").strip())
    name = reader.read_tilde_string().strip()

    # bottom_room top_room lifespan reset_mode
    parts = reader.next().split()
    bottom_room = int(parts[0])
    top_room = int(parts[1])
    lifespan = int(parts[2])
    reset_mode = int(parts[3])

    commands = []
    while not reader.eof():
        line = reader.next().strip()
        if not line or line.startswith("*"):
            continue
        if line == "S":
            break
        if line == "$":
            break
        cmd_type = line[0]
        if cmd_type not in "MOGEPDRT":
            continue
        # Parse numeric args after the command letter.
        # There may be a trailing comment (text after numbers).
        tokens = line[1:].split()
        args = []
        for t in tokens:
            try:
                args.append(int(t))
            except ValueError:
                break  # rest is comment
        commands.append({
            "type": cmd_type,
            "if_flag": args[0] if args else 0,
            "args": args[1:],
        })

    return {
        "zone_num": zone_num,
        "name": name,
        "bottom_room": bottom_room,
        "top_room": top_room,
        "lifespan": lifespan,
        "reset_mode": reset_mode,
        "commands": commands,
    }


def parse_wld(text):
    """Parse a .wld file, returning a list of room dicts."""
    rooms = []
    reader = LineReader(text)

    while not reader.eof():
        line = reader.peek()
        if line is None:
            break
        if line.strip() == "$":
            break
        if not line.startswith("#"):
            reader.next()
            continue

        vnum = int(reader.next().lstrip("#").strip())
        name = reader.read_tilde_string().strip()
        description = reader.read_tilde_string()

        # zone_num flags sector_type
        meta_line = reader.next().split()
        zone_num = int(meta_line[0])
        flags = decode_bitvector(meta_line[1])
        sector_type = int(meta_line[2]) if len(meta_line) > 2 else 0

        exits = []
        extra_descs = []

        while not reader.eof():
            line = reader.peek()
            if line is None:
                break
            line_stripped = line.strip()

            if line_stripped == "S":
                reader.next()
                break
            if line_stripped.startswith("#") or line_stripped == "$":
                break

            reader.next()

            if line_stripped.startswith("D"):
                direction = int(line_stripped[1:])
                desc = reader.read_tilde_string()
                keywords_str = reader.read_tilde_string()
                keywords = keywords_str.split() if keywords_str.strip() else []
                door_line = reader.next().split()
                door_flag = int(door_line[0])
                key_vnum = int(door_line[1])
                dest_room = int(door_line[2])
                exits.append({
                    "direction": direction,
                    "description": desc.strip(),
                    "keywords": keywords,
                    "door_flag": door_flag,
                    "key_vnum": key_vnum,
                    "dest_room": dest_room,
                })
            elif line_stripped == "E":
                kw_str = reader.read_tilde_string()
                desc = reader.read_tilde_string()
                keywords = kw_str.split() if kw_str.strip() else []
                if keywords and desc.strip():
                    extra_descs.append({
                        "keywords": keywords,
                        "description": desc.strip(),
                    })

        rooms.append({
            "vnum": vnum,
            "name": name,
            "description": description,
            "zone_num": zone_num,
            "flags": flags,
            "sector_type": sector_type,
            "exits": exits,
            "extra_descs": extra_descs,
        })

    return rooms


def parse_mob(text):
    """Parse a .mob file, returning a list of mob dicts."""
    mobs = []
    reader = LineReader(text)

    while not reader.eof():
        line = reader.peek()
        if line is None:
            break
        if line.strip() == "$":
            break
        if not line.startswith("#"):
            reader.next()
            continue

        vnum = int(reader.next().lstrip("#").strip())
        aliases_str = reader.read_tilde_string()
        aliases = aliases_str.split() if aliases_str.strip() else []
        short_desc = reader.read_tilde_string().strip()
        long_desc = reader.read_tilde_string().strip()
        detail_desc = reader.read_tilde_string().strip()

        # action_flags affect_flags alignment S|E
        flag_line = reader.next().split()
        action_flags = decode_bitvector(flag_line[0])
        affect_flags = decode_bitvector(flag_line[1])
        alignment = int(flag_line[2])
        mob_type = flag_line[3]  # "S" or "E"

        # level thac0 ac hp_dice bhd_dice
        stats_line = reader.next().split()
        level = int(stats_line[0])
        thac0 = int(stats_line[1])
        ac = int(stats_line[2])
        hp_dice = parse_dice(stats_line[3])
        bhd_dice = parse_dice(stats_line[4])

        # gold xp
        gold_line = reader.next().split()
        gold = int(gold_line[0])
        xp = int(gold_line[1])

        # load_pos default_pos sex
        pos_line = reader.next().split()
        load_pos = int(pos_line[0])
        default_pos = int(pos_line[1])
        sex = int(pos_line[2])

        e_specs = {}
        if mob_type == "E":
            while not reader.eof():
                line = reader.peek()
                if line is None:
                    break
                line_stripped = line.strip()
                if line_stripped == "E":
                    reader.next()
                    break
                if line_stripped.startswith("#") or line_stripped == "$":
                    break
                reader.next()
                if ":" in line_stripped:
                    key, val = line_stripped.split(":", 1)
                    e_specs[key.strip()] = val.strip()

        mobs.append({
            "vnum": vnum,
            "aliases": aliases,
            "short_desc": short_desc,
            "long_desc": long_desc,
            "detail_desc": detail_desc,
            "action_flags": action_flags,
            "affect_flags": affect_flags,
            "alignment": alignment,
            "mob_type": mob_type,
            "level": level,
            "thac0": thac0,
            "ac": ac,
            "max_hp": hp_dice,
            "bare_hand_damage": bhd_dice,
            "gold": gold,
            "xp": xp,
            "load_pos": load_pos,
            "default_pos": default_pos,
            "sex": sex,
            "e_specs": e_specs,
        })

    return mobs


def parse_obj(text):
    """Parse a .obj file, returning a list of object dicts."""
    objects = []
    reader = LineReader(text)

    while not reader.eof():
        line = reader.peek()
        if line is None:
            break
        if line.strip() == "$":
            break
        if not line.startswith("#"):
            reader.next()
            continue

        vnum = int(reader.next().lstrip("#").strip())
        aliases_str = reader.read_tilde_string()
        aliases = aliases_str.split() if aliases_str.strip() else []
        short_desc = reader.read_tilde_string().strip()
        long_desc = reader.read_tilde_string().strip()
        action_desc = reader.read_tilde_string().strip()

        # type effects_flags wear_flags
        type_line = reader.next().split()
        type_flag = int(type_line[0])
        effects_flags = decode_bitvector(type_line[1])
        wear_flags = decode_bitvector(type_line[2])

        # val0 val1 val2 val3
        val_line = reader.next().split()
        values = [int(v) for v in val_line[:4]]
        while len(values) < 4:
            values.append(0)

        # weight cost rent
        wcr_line = reader.next().split()
        weight = int(wcr_line[0])
        cost = int(wcr_line[1])
        rent = int(wcr_line[2])

        affects = []
        extra_descs = []

        while not reader.eof():
            line = reader.peek()
            if line is None:
                break
            line_stripped = line.strip()
            if line_stripped.startswith("#") or line_stripped == "$":
                break
            reader.next()
            if line_stripped == "A":
                aff_line = reader.next().split()
                location = int(aff_line[0])
                value = int(aff_line[1])
                affects.append({"location": location, "value": value})
            elif line_stripped == "E":
                kw_str = reader.read_tilde_string()
                desc = reader.read_tilde_string()
                keywords = kw_str.split() if kw_str.strip() else []
                if keywords and desc.strip():
                    extra_descs.append({
                        "keywords": keywords,
                        "description": desc.strip(),
                    })

        objects.append({
            "vnum": vnum,
            "aliases": aliases,
            "short_desc": short_desc,
            "long_desc": long_desc,
            "action_desc": action_desc,
            "type_flag": type_flag,
            "effects_flags": effects_flags,
            "wear_flags": wear_flags,
            "values": values,
            "weight": weight,
            "cost": cost,
            "rent": rent,
            "affects": affects,
            "extra_descs": extra_descs,
        })

    return objects


# ---------------------------------------------------------------------------
# Zone metadata and vnum mapping
# ---------------------------------------------------------------------------

def build_zone_info(parsed_zones):
    zones = {}
    for z in parsed_zones:
        slug = slugify(z["name"])
        slug = ZONE_SLUG_OVERRIDES.get(slug, slug)
        zones[z["zone_num"]] = {
            "zone_num": z["zone_num"],
            "name": z["name"],
            "slug": slug,
            "lifespan": z["lifespan"],
            "reset_mode": z["reset_mode"],
            "bottom_room": z["bottom_room"],
            "top_room": z["top_room"],
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


# ---------------------------------------------------------------------------
# Conversion: rooms
# ---------------------------------------------------------------------------

def convert_exit(ex, source_zone_slug, v2z, zones, known_obj_vnums):
    dest_vnum = ex["dest_room"]
    if dest_vnum < 0:
        return None

    dest_zone_slug = v2z.get(dest_vnum, source_zone_slug)

    result = {
        "room_id": f"{dest_zone_slug}-{dest_vnum}",
    }

    if dest_zone_slug != source_zone_slug:
        result["zone_id"] = dest_zone_slug

    if ex["description"]:
        result["description"] = collapse_newlines(ex["description"])

    door_flag = ex["door_flag"]
    if door_flag > 0:
        keywords = ex["keywords"]
        name = keywords[0] if keywords else "door"
        closure = {
            "name": name,
            "closed": True,
        }
        key_vnum = ex["key_vnum"]
        if key_vnum > 0 and key_vnum in known_obj_vnums:
            lock = {
                "key_id": vnum_to_id(key_vnum, v2z, zones),
                "locked": True,
            }
            if door_flag == 2:
                lock["pickproof"] = True
            closure["lock"] = lock
        result["closure"] = closure

    return result


def convert_room(parsed_room, zone_slug, v2z, zones, known_obj_vnums):
    perks = []

    flag_names = bitvector_to_names(parsed_room["flags"], ROOM_FLAG_BITS)
    skipped_flags = []
    for name in flag_names:
        body = ROOM_FLAGS.get(name)
        if body is None:
            skipped_flags.append(name)
            continue
        perks.append({"type": "grant", **body})

    sector = SECTOR_TYPES.get(parsed_room["sector_type"], "")
    if (sector_body := SECTOR_FLAGS.get(sector)):
        perks.append({"type": "grant", **sector_body})

    exits = {}
    for ex in parsed_room["exits"]:
        direction = DIRECTIONS.get(ex["direction"])
        if direction is None:
            continue
        converted = convert_exit(ex, zone_slug, v2z, zones, known_obj_vnums)
        if converted is None:
            continue
        exits[direction] = converted

    extra_descs = []
    for ed in parsed_room["extra_descs"]:
        desc = collapse_newlines(ed["description"])
        if ed["keywords"] and desc:
            extra_descs.append({"keywords": ed["keywords"], "description": desc})

    vnum = parsed_room["vnum"]
    room = {
        "name": parsed_room["name"],
        "description": collapse_newlines(parsed_room["description"]),
        "zone_id": zone_slug,
        "exits": exits,
    }

    if perks:
        room["perks"] = perks
    if extra_descs:
        room["extra_descs"] = extra_descs

    unused = {}
    if sector:
        unused["sector_type"] = sector
    if skipped_flags:
        unused["flags"] = skipped_flags

    missing_keys = {}
    for ex in parsed_room["exits"]:
        if ex["door_flag"] > 0 and ex["key_vnum"] > 0 and ex["key_vnum"] not in known_obj_vnums:
            direction = DIRECTIONS.get(ex["direction"])
            if direction:
                missing_keys[direction] = ex["key_vnum"]
    if missing_keys:
        unused["missing_exit_keys"] = missing_keys

    result = {
        "version": 1,
        "id": f"{zone_slug}-{vnum}",
        "spec": room,
    }

    if unused:
        result["circlemud_unused"] = unused

    return result


# ---------------------------------------------------------------------------
# Conversion: mobiles
# ---------------------------------------------------------------------------

def convert_mob(parsed_mob, zone_slug, v2z, zones):
    perks = []
    flags = []

    action_names = bitvector_to_names(parsed_mob["action_flags"], MOB_ACTION_BITS)
    skipped_flags = []
    for name in action_names:
        if name in MOB_FLAGS:
            flags.append(MOB_FLAGS[name])
        elif name not in ("SPEC", "ISNPC"):
            skipped_flags.append(name)

    affect_names = bitvector_to_names(parsed_mob["affect_flags"], MOB_AFFECT_BITS)
    skipped_affects = []
    for name in affect_names:
        if name == "SANCTUARY":
            perks.append({"type": "modifier", "key": "core.defense.all.absorb.pct", "value": 50})
        elif name in AFFECT_GRANTS:
            perks.append({"type": "grant", **AFFECT_GRANTS[name]})
        elif name not in ("BLIND", "DETECT_ALIGN", "DETECT_MAGIC", "GROUP",
                          "CURSE", "POISON", "SLEEP"):
            skipped_affects.append(name)

    # Armor class: CircleMUD uses descending AC (10=unarmored, lower=better).
    # Convert to ascending (0=unarmored, higher=better): ascending = 10 - descending.
    ac = parsed_mob["ac"]
    ascending_ac = 10 - ac
    if ascending_ac != 0:
        perks.append({"type": "modifier", "key": "core.combat.ac.flat", "value": ascending_ac})

    # Bare hand damage
    bhd = parsed_mob["bare_hand_damage"]
    if bhd["dice"] > 0 and bhd["sides"] > 0:
        dice_expr = f"{bhd['dice']}d{bhd['sides']}"
        if bhd["bonus"] > 0:
            dice_expr += f"+{bhd['bonus']}"
        perks.append({"type": "grant", "key": "attack", "arg": dice_expr})
        perks.append({"type": "grant", "key": "auto_use", "arg": "attack:1"})

    # Max HP
    mhp = parsed_mob["max_hp"]
    if mhp:
        hp = mhp["dice"] * mhp["sides"] + mhp["bonus"]
        if hp > 0:
            perks.append({"type": "modifier", "key": "core.resource.hp.max", "value": hp})

    # THAC0 -> attack bonus
    thac0 = parsed_mob["thac0"]
    if thac0 != 20:
        attack_bonus = 20 - thac0
        if attack_bonus != 0:
            perks.append({"type": "modifier", "key": "core.combat.attack.flat", "value": attack_bonus})

    vnum = parsed_mob["vnum"]
    mid = vnum_to_id(vnum, v2z, zones)

    mobile = {
        "aliases": parsed_mob["aliases"],
        "short_desc": parsed_mob["short_desc"],
        "long_desc": parsed_mob["long_desc"],
        "detailed_desc": collapse_newlines(parsed_mob["detail_desc"]),
        "level": parsed_mob["level"],
    }

    if perks:
        mobile["perks"] = perks
    if flags:
        mobile["flags"] = flags

    if parsed_mob["xp"] > 0:
        mobile["exp_reward"] = parsed_mob["xp"]

    unused = {}
    if parsed_mob["alignment"] != 0:
        unused["alignment"] = parsed_mob["alignment"]
    if parsed_mob["gold"] != 0:
        unused["gold"] = parsed_mob["gold"]
    gender = SEX_MAP.get(parsed_mob["sex"], "")
    if gender:
        unused["gender"] = gender
    if skipped_flags:
        unused["flags"] = skipped_flags
    if skipped_affects:
        unused["affects"] = skipped_affects

    result = {
        "version": 1,
        "id": mid,
        "spec": mobile,
    }

    if unused:
        result["circlemud_unused"] = unused

    return result


# ---------------------------------------------------------------------------
# Conversion: objects
# ---------------------------------------------------------------------------

def convert_object(parsed_obj, zone_slug, v2z, zones, known_obj_vnums):
    flags = []
    perks = []

    type_name = OBJ_TYPE_MAP.get(parsed_obj["type_flag"], "OTHER")
    values = parsed_obj["values"]

    # Wear slots
    wear_names = bitvector_to_names(parsed_obj["wear_flags"], OBJ_WEAR_BITS)
    has_take = "WEAR_TAKE" in wear_names

    slots = []
    for wn in wear_names:
        slot = WEAR_SLOTS.get(wn)
        if slot:
            slots.append(slot)

    if slots:
        flags.append("wearable")
    if not has_take and not slots:
        flags.append("immobile")

    # Effect flags
    effect_names = bitvector_to_names(parsed_obj["effects_flags"], OBJ_EFFECT_BITS)
    skipped_effects = []
    for name in effect_names:
        if name in EFFECT_FLAGS:
            flags.append(EFFECT_FLAGS[name])
        elif name in EFFECT_SKIP:
            skipped_effects.append(name)

    # Type-specific handling
    unused = {}

    if type_name == "WEAPON":
        dice = values[1]
        sides = values[2]
        if dice > 0 and sides > 0:
            perks.append({"type": "grant", "key": "attack", "arg": f"{dice}d{sides}"})
        wtype = values[3]
        if wtype in WEAPON_TYPES:
            unused["weapon_type"] = WEAPON_TYPES[wtype]

    elif type_name == "ARMOR":
        ac = values[0]
        if ac != 0:
            perks.append({"type": "modifier", "key": "core.combat.ac.flat", "value": ac})

    elif type_name == "LIGHT":
        burn_time = values[2]
        if burn_time != 0:
            perks.append({"type": "grant", "key": "ignore_restriction", "arg": "room_dark"})
        if burn_time > 0:
            unused["burn_time_hours"] = burn_time

    elif type_name == "CONTAINER":
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
                        "key_id": vnum_to_id(key_vnum, v2z, zones),
                        "locked": True,
                    }
                    if cflag & CONTAINER_PICKPROOF:
                        lock["pickproof"] = True
                    closure["lock"] = lock
                else:
                    unused["keyless_lock"] = True
                    if key_vnum > 0:
                        unused["keyless_lock_missing_key"] = key_vnum
                    if cflag & CONTAINER_PICKPROOF:
                        unused["keyless_lock_pickproof"] = True
            unused["closure"] = closure

    # Affect fields -> perks
    for aff in parsed_obj["affects"]:
        loc_name = AFFECT_LOCATION_MAP.get(aff["location"], "")
        key = AFFECT_KEYS.get(loc_name)
        if key and aff["value"] != 0:
            # APPLY_AC is a delta to descending AC (negative = better).
            # Negate to get ascending delta (positive = better).
            value = -aff["value"] if loc_name == "AC" else aff["value"]
            perks.append({"type": "modifier", "key": key, "value": value})

    # Extra descriptions — promote alias-matching to detailed_desc
    aliases_set = set(a.lower() for a in parsed_obj["aliases"])
    detailed_desc = ""
    extra_descs = []
    for ed in parsed_obj["extra_descs"]:
        keywords = ed["keywords"]
        desc = collapse_newlines(ed["description"])
        if not keywords or not desc:
            continue
        ed_kw_set = set(k.lower() for k in keywords)
        if not detailed_desc and ed_kw_set & aliases_set:
            detailed_desc = desc
        else:
            extra_descs.append({"keywords": keywords, "description": desc})

    # Build asset
    vnum = parsed_obj["vnum"]
    oid = vnum_to_id(vnum, v2z, zones)

    obj = {
        "aliases": parsed_obj["aliases"],
        "short_desc": parsed_obj["short_desc"],
    }

    long_desc = collapse_newlines(parsed_obj["long_desc"])
    if long_desc:
        obj["long_desc"] = long_desc

    if detailed_desc:
        obj["detailed_desc"] = detailed_desc
    elif long_desc:
        obj["detailed_desc"] = long_desc

    if flags:
        obj["flags"] = flags
    if slots:
        obj["wear_slots"] = slots
    if perks:
        obj["perks"] = perks
    if extra_descs:
        obj["extra_descs"] = extra_descs

    # circlemud_unused
    if parsed_obj["cost"] != 0:
        unused["cost"] = parsed_obj["cost"]
    if parsed_obj["rent"] != 0:
        unused["rent"] = parsed_obj["rent"]
    if parsed_obj["weight"] != 0:
        unused["weight"] = parsed_obj["weight"]
    if skipped_effects:
        unused["effects"] = skipped_effects
    if type_name not in ("WEAPON", "ARMOR", "LIGHT", "CONTAINER", "KEY",
                          "TREASURE", "OTHER", "TRASH"):
        unused["type"] = type_name
        unused["values"] = values

    # Move closure from unused into spec proper.
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
# Conversion: zones
# ---------------------------------------------------------------------------

def convert_zone(zone_info):
    reset_mode = RESET_MODES.get(zone_info["reset_mode"], "lifespan")
    zone = {
        "name": zone_info["name"],
        "reset_mode": reset_mode,
    }
    if reset_mode != "never" and zone_info["lifespan"] > 0:
        zone["lifespan"] = f"{zone_info['lifespan']}m"

    result = {
        "version": 1,
        "id": zone_info["slug"],
        "spec": zone,
    }

    creators = ZONE_CREATORS.get(zone_info["slug"])
    if creators:
        result["creators"] = creators

    return result


# ---------------------------------------------------------------------------
# Zone command processing
# ---------------------------------------------------------------------------

def process_zone_commands(parsed_zones, v2z, zones):
    """Walk zone reset commands to build spawn and placement data."""
    room_mob_spawns = {}     # room_id -> [mob_id, ...]
    room_obj_spawns = {}     # room_id -> [ObjectSpawn, ...]
    mob_inventory = {}       # mob_id -> [{"object_id": ...}, ...]
    mob_equipment = {}       # mob_id -> [{"slot": ..., "object_id": ...}, ...]

    seen_equip_slots = {}    # mid -> set of slots already filled
    seen_inventory = set()   # (mid, oid)

    for zone in parsed_zones:
        current_mob_vnum = None

        for cmd in zone["commands"]:
            ctype = cmd["type"]
            args = cmd["args"]

            if ctype == "M" and len(args) >= 3:
                mob_vnum, max_exist, room_vnum = args[0], args[1], args[2]
                current_mob_vnum = mob_vnum
                mid = vnum_to_id(mob_vnum, v2z, zones)
                rid = vnum_to_id(room_vnum, v2z, zones)
                room_mob_spawns.setdefault(rid, []).append(mid)

            elif ctype == "O" and len(args) >= 3:
                obj_vnum, max_exist, room_vnum = args[0], args[1], args[2]
                oid = vnum_to_id(obj_vnum, v2z, zones)
                rid = vnum_to_id(room_vnum, v2z, zones)
                spawn = {"object_id": oid}
                room_obj_spawns.setdefault(rid, []).append(spawn)

            elif ctype == "G" and len(args) >= 2 and current_mob_vnum is not None:
                obj_vnum = args[0]
                mid = vnum_to_id(current_mob_vnum, v2z, zones)
                oid = vnum_to_id(obj_vnum, v2z, zones)
                key = (mid, oid)
                if key not in seen_inventory:
                    seen_inventory.add(key)
                    mob_inventory.setdefault(mid, []).append({"object_id": oid})

            elif ctype == "E" and len(args) >= 3 and current_mob_vnum is not None:
                obj_vnum, max_exist, eq_pos = args[0], args[1], args[2]
                slot = EQUIP_POSITION_MAP.get(eq_pos)
                if slot:
                    mid = vnum_to_id(current_mob_vnum, v2z, zones)
                    oid = vnum_to_id(obj_vnum, v2z, zones)
                    filled = seen_equip_slots.setdefault(mid, set())
                    if slot not in filled:
                        filled.add(slot)
                        mob_equipment.setdefault(mid, []).append({
                            "slot": slot,
                            "object_id": oid,
                        })

            elif ctype == "P" and len(args) >= 3:
                obj_vnum, max_exist, container_vnum = args[0], args[1], args[2]
                oid = vnum_to_id(obj_vnum, v2z, zones)
                cid = vnum_to_id(container_vnum, v2z, zones)
                # Find last loaded instance of the container in room spawns.
                found = False
                for rid in reversed(list(room_obj_spawns.keys())):
                    for spawn in reversed(room_obj_spawns[rid]):
                        if spawn["object_id"] == cid:
                            spawn.setdefault("contents", []).append({"object_id": oid})
                            found = True
                            break
                    if found:
                        break

    return room_mob_spawns, room_obj_spawns, mob_inventory, mob_equipment


# ---------------------------------------------------------------------------
# File output helpers
# ---------------------------------------------------------------------------

def write_json(dirpath, filename, data):
    os.makedirs(dirpath, exist_ok=True)
    filepath = os.path.join(dirpath, filename)
    with open(filepath, "w") as f:
        json.dump(data, f, indent=4)
        f.write("\n")


def clean_dir(dirpath):
    """Remove all .json files under dirpath recursively."""
    if not os.path.isdir(dirpath):
        return 0
    count = 0
    for root, dirs, files in os.walk(dirpath):
        for fname in files:
            if fname.endswith(".json"):
                os.remove(os.path.join(root, fname))
                count += 1
    # Remove empty subdirectories.
    for root, dirs, files in os.walk(dirpath, topdown=False):
        for d in dirs:
            dp = os.path.join(root, d)
            try:
                os.rmdir(dp)
            except OSError:
                pass
    return count


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    rooms_dir = os.path.join(OUTPUT_DIR, "rooms")
    mobiles_dir = os.path.join(OUTPUT_DIR, "mobiles")
    objects_dir = os.path.join(OUTPUT_DIR, "objects")
    zones_dir = os.path.join(OUTPUT_DIR, "zones")

    # 1. Fetch index files to discover what to load.
    print("Fetching index files...", flush=True)
    zon_files = fetch_index("zon")
    wld_files = fetch_index("wld")
    mob_files = fetch_index("mob")
    obj_files = fetch_index("obj")
    print(f"  zon: {len(zon_files)}, wld: {len(wld_files)}, mob: {len(mob_files)}, obj: {len(obj_files)}")

    # 2. Parse all zone files first (needed for zone metadata).
    print("Parsing zone files...", flush=True)
    parsed_zones = []
    for zn in zon_files:
        text = fetch_text(f"zon/{zn}.zon")
        parsed_zones.append(parse_zon(text))
    print(f"  {len(parsed_zones)} zones")

    zones = build_zone_info(parsed_zones)
    v2z = build_vnum_to_zone(zones)

    # 3. Parse all source files.
    print("Parsing world files...", flush=True)
    all_rooms = {}
    for fn in wld_files:
        text = fetch_text(f"wld/{fn}.wld")
        rooms = parse_wld(text)
        all_rooms[fn] = rooms
        # Register vnums — use zone range to find the correct slug.
        for r in rooms:
            if r["vnum"] not in v2z:
                # Room outside declared range; find zone by vnum // 100.
                zone_num = r["vnum"] // 100
                if zone_num in zones:
                    v2z[r["vnum"]] = zones[zone_num]["slug"]
        print(f"  wld/{fn}.wld: {len(rooms)} rooms")

    print("Parsing mobile files...", flush=True)
    all_mobs = {}
    for fn in mob_files:
        try:
            text = fetch_text(f"mob/{fn}.mob")
        except Exception as e:
            print(f"  mob/{fn}.mob: ERROR {e}")
            continue
        mobs = parse_mob(text)
        all_mobs[fn] = mobs
        # Register mob vnums.
        for m in mobs:
            if m["vnum"] not in v2z:
                zone_num = m["vnum"] // 100
                if zone_num in zones:
                    v2z[m["vnum"]] = zones[zone_num]["slug"]
        print(f"  mob/{fn}.mob: {len(mobs)} mobs")

    print("Parsing object files...", flush=True)
    all_objs = {}
    for fn in obj_files:
        try:
            text = fetch_text(f"obj/{fn}.obj")
        except Exception as e:
            print(f"  obj/{fn}.obj: ERROR {e}")
            continue
        objs = parse_obj(text)
        all_objs[fn] = objs
        # Register object vnums.
        for o in objs:
            if o["vnum"] not in v2z:
                zone_num = o["vnum"] // 100
                if zone_num in zones:
                    v2z[o["vnum"]] = zones[zone_num]["slug"]
        print(f"  obj/{fn}.obj: {len(objs)} objects")

    known_obj_vnums = set()
    for objs in all_objs.values():
        for o in objs:
            known_obj_vnums.add(o["vnum"])

    # 4. Process zone commands to get spawn/placement data.
    print("Processing zone commands...", flush=True)
    room_mob_spawns, room_obj_spawns, mob_inventory, mob_equipment = \
        process_zone_commands(parsed_zones, v2z, zones)

    # 5. Clean existing output and write fresh.
    for d in [zones_dir, rooms_dir, mobiles_dir, objects_dir]:
        n = clean_dir(d)
        if n:
            print(f"  Cleaned {n} files from {d}")

    # 6. Write zone files.
    os.makedirs(zones_dir, exist_ok=True)
    for z in zones.values():
        out = convert_zone(z)
        write_json(zones_dir, f"{z['slug']}.json", out)
    print(f"Wrote {len(zones)} zone files")

    # 7. Write room files.
    total_rooms = 0
    for fn, rooms in all_rooms.items():
        for r in rooms:
            vnum = r["vnum"]
            zone_slug = v2z.get(vnum)
            if zone_slug is None:
                print(f"  WARN: no zone for room {vnum}, skipping")
                continue
            out = convert_room(r, zone_slug, v2z, zones, known_obj_vnums)
            rid = out["id"]
            # Add mob spawns.
            if rid in room_mob_spawns:
                out["spec"]["mobile_spawns"] = room_mob_spawns[rid]
            # Add object spawns.
            if rid in room_obj_spawns:
                out["spec"]["object_spawns"] = room_obj_spawns[rid]
            zone_rooms_dir = os.path.join(rooms_dir, zone_slug)
            write_json(zone_rooms_dir, f"{rid}.json", out)
            total_rooms += 1
    print(f"Wrote {total_rooms} room files")

    # 8. Write mobile files.
    total_mobs = 0
    for fn, mobs in all_mobs.items():
        for m in mobs:
            vnum = m["vnum"]
            zone_slug = v2z.get(vnum)
            if zone_slug is None:
                zone_slug = zones.get(fn, {}).get("slug")
            if zone_slug is None:
                print(f"  WARN: no zone for mob {vnum}, skipping")
                continue
            out = convert_mob(m, zone_slug, v2z, zones)
            mid = out["id"]
            if mid in mob_inventory:
                out["spec"]["inventory"] = mob_inventory[mid]
            if mid in mob_equipment:
                out["spec"]["equipment"] = mob_equipment[mid]
            zone_mob_dir = os.path.join(mobiles_dir, zone_slug)
            write_json(zone_mob_dir, f"{mid}.json", out)
            total_mobs += 1
    print(f"Wrote {total_mobs} mob files")

    # 9. Write object files.
    total_objs = 0
    for fn, objs in all_objs.items():
        for o in objs:
            vnum = o["vnum"]
            zone_slug = v2z.get(vnum)
            if zone_slug is None:
                zone_slug = zones.get(fn, {}).get("slug")
            if zone_slug is None:
                print(f"  WARN: no zone for obj {vnum}, skipping")
                continue
            out = convert_object(o, zone_slug, v2z, zones, known_obj_vnums)
            zone_obj_dir = os.path.join(objects_dir, zone_slug)
            write_json(zone_obj_dir, f"{out['id']}.json", out)
            total_objs += 1
    print(f"Wrote {total_objs} object files")

    print(f"\nTotal: {len(zones)} zones, {total_rooms} rooms, {total_mobs} mobs, {total_objs} objects")


if __name__ == "__main__":
    main()

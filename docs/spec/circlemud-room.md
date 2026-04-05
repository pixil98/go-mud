3\. World (Room) Files
----------------------

3.1 The Format of a Room
------------------------

The format of a room is:

> `* * *  #<virtual number> <room name>~ <Room Description> ~ <zone number> <room bitvector> <sector type> {Zero or more Direction Fields and/or Extra Descriptions} S  * * *`

There can be between 0 and 6 Direction Fields. There should not be more than one Direction Field for a particular direction. No Extra Descriptions are required but an unlimited number are allowed. Each room is terminated with the literal letter `S`.

**Virtual Number**

This number is critical; it is the identity of the room within the game. All other files will use this number to refer to this room. From within the game, this number can be used with \`\`goto'' to go to this room. The virtual numbers must appear in increasing order in the world file.

**Room Name**

This string is the room's title, which is displayed before the room description when players look at the room, or displayed alone if players are using \`\`brief'').

**Room Description**

The description of the room seen when they type \`\`look,'' or when they enter the room with brief mode off.

**Zone Number**

This number is obsolete and no longer used. Historically it contained the zone number of the current room but it is currently ignored. It is maintained as part of the format for backwards compatibility.

**Room Bitvector**

A bitvector (see section [Using Bitvectors](building-2.html#bitvectors)), with the following values:

> `* * *  1     a   DARK           Room is dark. 2     b   DEATH          Room is a death trap; char ``dies'' (no xp lost). 4     c   NOMOB          MOBs (monsters) cannot enter room. 8     d   INDOORS        Room is indoors. 16    e   PEACEFUL       Room is peaceful (violence not allowed). 32    f   SOUNDPROOF     Shouts, gossips, etc. won't be heard in room. 64    g   NOTRACK        ``track'' can't find a path through this room. 128   h   NOMAGIC        All magic attempted in this room will fail. 256   i   TUNNEL         Only one person allowed in room at a time. 512   j   PRIVATE        Cannot teleport in or GOTO if two people here. 1024  k   GODROOM        Only LVL_GOD and above allowed to enter. 2048  l   HOUSE          Reserved for internal use.  Do not set.  4096  m   HOUSE_CRASH    Reserved for internal use.  Do not set.  8192  n   ATRIUM         Reserved for internal use.  Do not set.       16384 o   OLC            Reserved for internal use.  Do not set.  32768 p   BFS_MARK       Reserved for internal use.  Do not set.   * * *` 

**Sector Type**

A single number (_not_ a bitvector) defining the type of terrain in the room. Note that this value is not the number of movement points needed but just a number to identify the sector type (the movement loss is controlled by the array `movement_loss[]` in the file `constants.c`). The Sector Type can be one of the following:

> `* * *  0    INSIDE         Indoors (small number of move points needed). 1    CITY           The streets of a city. 2    FIELD          An open field. 3    FOREST         A dense forest. 4    HILLS          Low foothills. 5    MOUNTAIN       Steep mountain regions. 6    WATER_SWIM     Water (swimmable). 7    WATER_NOSWIM   Unswimmable water - boat required for passage. 8    UNDERWATER     Underwater. 9    FLYING         Wheee!  * * *`

**Direction Fields and Extra Descriptions**

This section defines the room's exits, if any, as well as any extra descriptions such as signs or strange objects that might be in the room. This section can be empty if the room has no exits and no extra descriptions. Otherwise, it can have any number of `D` (Direction Field) and `E` (Extra Description) sections, in any order. After all exits and extra descriptions have been listed, the end of the room is signaled with the letter `S`. The Direction Fields and Extra Descriptions are described in more detail in the following sections.

3.2 The Direction Field
-----------------------

The general format of a direction field is:

> `* * *  D<direction number> <general description> ~ <keyword list>~ <door flag> <key number> <room linked>  * * *`

**Direction Number**

The compass direction that this Direction Field describes. It must be one of the following numbers:

>      `0    North      1    East      2    South      3    West      4    Up      5    Down` 

**General Description**

The description shown to the player when she types \`\`look <direction>.'' This should not be confused with the room description itself. Unlike the room description which is automatically viewed when a player walks into a room, the General Description of an exit is only seen when a player looks in the direction of the exit (e.g., \`\`look north'').

**Keyword List**

A list of acceptable terms that can be used to manipulate the door with commands such as \`\`open,'' \`\`close,'' \`\`lock,'' \`\`unlock,'' etc. The list should be separated by spaces, e.g.:

door oak big~

**Door Flag**

Can take one of three values (0, 1 or 2):

**0**

An unrestricted exit that has no door, or a special door cannot be opened or closed with the \`\`open'' and \`\`close'' commands. The latter is useful for secret doors, trap doors, or other doors that are opened and closed by something other than the normal commands, like a special procedure assigned to the room or an object in the room.

**1**

Normal doors that can be opened, closed, locked, unlocked, and picked.

**2**

Pickproof doors: if locked, can be opened only with the key.

The initial state of all doors is open, but doors can be opened, closed, and locked automatically when zones reset (see the zone file documentation for details).

**Key Number**

The virtual number of the key required to lock and unlock the door in the direction given. A value of -1 means that there is no keyhole; i.e., no key will open this door. If the Door Flag for this door is 0, the Key Number is ignored.

**Room Linked**

The virtual number of the room to which this exit leads. If this number is -1 (NOWHERE), the exit will not actually lead anywhere; useful if you'd like the exit to show up on \`\`exits,'' or if you'd like to add a description for \`\`look <direction>'' without actually adding an exit in that direction.

3.3 Room Extra Descriptions
---------------------------

Extra descriptions are used to make rooms more interesting, and make them more interactive. Extra descriptions are accessed by players when they type \`\`look at <thing>,'' where <thing> is any word you choose. For example, you might write a room description which includes the tantalizing sentence, \`\`The wall looks strange here.'' Using extra descriptions, players could then see additional detail by typing \`\`look at wall.'' There can be an unlimited number of Extra Descriptions in each room.

The format of an extra description is simple:

> `* * *  E <keyword list>~ <description text> ~  * * *`

**Keyword List**

A space-separated list of keywords which will access the description in this `E` section.

**Description Text**

The text that will be displayed when a player types \`\`look <keyword>,'' where <keyword> is one of the keywords specified in the Keyword List of this `E` section.

3.4 World File Example
----------------------

Here's a sample entry from a CircleMUD world file:

> `#18629 The Red Room~    It takes you a moment to realize that the red glow here is coming from a round portal on the floor.  It looks almost as if someone had painted a picture of a dirt running through a field on the floor of this room.  Oddly enough, it is so realistic you can feel the wind in the field coming out of the picture. ~ 186 ad 0 D0 You see a big room up there. ~ ~ 0 -1 18620 D1 You see a small room. ~ oak door~ 1 18000 18630 E portal floor~ It looks as if you could go down into it... but you can't be sure of where you will end up, or if you can get back. ~ S`

This room is virtual number 18629, called \`\`The Red Room.'' It is dark and indoors, with an \`\`INDOORS'' sector type. It has an exit north and east. The north exit leads to room 18620; if a player types \`\`look north'' it will say \`\`You see a big room up there.'' The exit east is a normal, pickable door that leads to room 18630 and which takes key number 18000. There is one extra description for \`\`portal'' and \`\`floor.''

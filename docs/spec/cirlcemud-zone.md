6\. Zone Files
--------------

Zone files are the files that control how areas are configured and how they reset. They integrate the mobiles, objects, and rooms to create an inhabited world.

A zone file contains certain initial information (specified below), followed by a series of reset commands. Each time a zone is reset, the server executes all the commands in order from beginning to end. All zones are reset when the server first boots, and periodically reset again while the game is running.

6.1 The Format of a Zone File
-----------------------------

> `* * *  #<virtual number> <zone name>~ <top room number> <lifespan> <reset mode> {zero or more zone commands} S  * * *`

Lines starting with `*` are considered comments and ignored. Zone commands themselves may also be followed by a comment delimited by a single `*`. The zone's commands must then be terminated by the literal letter `S`.

**Virtual Number**

An arbitrary number used to identify the zone. Zone numbers are traditionally the room numbers of the zone divided by 100; for example, Midgaard, which consists of rooms 3000 through 3099, is zone 30.

**Zone Name**

A label given to the zone so that it can be identified in system logs.

**Top Room Number**

The highest numbered room belonging to this zone. A room with virtual number V belongs to zone N if TopRoom(zone N-1) < V <= TopRoom(zone N) for all N > 0. Rooms belong to zone 0 if their number is between 0 and the top of zone 0.

**Lifespan**

The number of real-time minutes between zone resets for this zone. When the age of the zone (measured in minutes since the last time that zone has been reset) reaches the zone's lifespan, the zone is queued for reset. The zone is then reset when it reaches the front of the queue, and the conditions of the Reset Mode (see below) are satisfied.

**Reset Mode**

Can take one of three values (0, 1, or 2):

**0**

Never reset the zone. In this case, the age of the zone is never updated, and it will never be queued for reset. Thus, the value of the Lifespan is effectively ignored.

**1**

Reset the zone only after it reaches its Lifespan _and_ after the zone becomes deserted, i.e. as soon as there are no players located within the zone (checked once every minute). This can make a zone more \`\`fair'' because it will keep the hard mobs from reappearing in the zone until everyone leaves, but on a busy MUD it can prevent a zone from ever being reset since the zone may never stay empty for more than one minute.

**2**

Reset the zone as soon as it reaches its Lifespan, regardless of who or what is in it. This is the most commonly used Reset Mode.

6.2 Zone Commands
-----------------

Each command consists of a letter, identifying the command-type, followed by three or four arguments. The first argument, common to all the commands, is called the \`\`if-flag.'' If the if-flag for a command is 1, that command is only executed if the command immediately before it was executed as well. If the if-flag is 0, the command is always executed. If-flags are useful for things like equipping mobiles--you don't want to try to equip a mobile that has not been loaded.

Commands that load mobiles and objects also include a \`\`max existing'' argument. This specifies the maximum number of copies of the mobile or object that are allowed to exist in the entire world at once. If the number currently existing is greater than or equal to the \`\`max existing'' limit, the command is not executed.

The valid zone-reset commands are M, O, G, E, P, D, and R.

**M: load a mobile**

Format: M <if-flag> <mob vnum> <max existing> <room vnum>

Mob vnum is the vnum of the mob to be loaded. Room vnum is the vnum of the room in which the mob should be placed. The mob will be loaded into the room.

**O: load an object**

Format: O <if-flag> <obj vnum> <max existing> <room vnum>

Obj vnum is the vnum of the obj to be loaded. Room vnum is the vnum of the room in which the obj should be placed. The object will be loaded and left lying on the ground.

**G: give object to mobile**

Format: G <if-flag> <obj vnum> <max existing>

Obj vnum is the vnum of the obj to be given. The object will be loaded and placed in the inventory of the last mobile loaded with an \`\`M'' command.

This command will usually be used with an if-flag of 1, since attempting to give an object to a non-existing mobile will result in an error.

**E: equip mobile with object**

Format: E <if-flag> <obj vnum> <max existing> <equipment position>

Obj vnum is the vnum of the obj to be equipped. The object will be loaded and added to the equipment list of the last mobile loaded with an \`\`M'' command. Equipment Position should be one of the following:

>           `0    Used as light           1    Worn on right finger           2    Worn on left finger           3    First object worn around neck           4    Second object worn around neck           5    Worn on body           6    Worn on head           7    Worn on legs           8    Worn on feet           9    Worn on hands           10   Worn on arms           11   Worn as shield           12   Worn about body           13   Worn around waist           14   Worn around right wrist           15   Worn around left wrist           16   Wielded as a weapon           17   Held`

This command will usually be used with an if-flag of 1, since attempting to give an object to a non-existing mobile will result in an error.

**P: put object in object**

Format: P <if-flag> <obj vnum 1> <max existing> <obj vnum 2>

An object with Obj Vnum 1 will be loaded, and placed inside of the copy of Obj Vnum 2 most recently loaded.

This command will usually be used with an if-flag of 1, since attempting to put an object inside of a non-existing object will result in an error.

**D: set the state of a door**

Format: D <if-flag> <room vnum> <exit num> <state>

Room vnum is the virtual number of the room with the door to be set. Exit num being one of:

>           `0    North           1    East           2    South           3    West           4    Up           5    Down` 

State being one of:

>           `0    Open           1    Closed           2    Closed and locked`

Care should be taken to set both sides of a door correctly. Closing the north exit of one room does not automatically close the south exit of the room on the other side of the door.

**R: remove object from room**

Format: R <if-flag> <room vnum> <obj vnum>

If an object with vnum Obj Vnum exists in the room with vnum Room Vnum, it will be removed from the room and purged.

6.3 Zone File Example
---------------------

A sample zone file annotated with comments follows.

> `#30                           * This is zone number 30 Northern Midgaard Main City~  * The name of the zone 3099 15 2                     * Top of zone is room #3099; it resets every *                             * 15 minutes; resets regardless of people * * Mobile M 0 3010 1 3062         * Load the Postmaster to room 3062 * Shopkeepers M 0 3003 1 3011         Load the Weaponsmith into room 3011 * Now, give the weaponsmith items (to be placed in his inventory) * max 100 of each of these objects can exist at a time in the world G 1 3020 100                    Dagger G 1 3021 100                    Small Sword G 1 3022 100                    Long Sword G 1 3023 100                    Wooden Club G 1 3024 100                    Warhammer G 1 3025 100                    Flail * and lastly, give him a long sword to wield E 1 3022 100 16                 Long Sword * Load Boards O 0 3099 2 3000         Mortal Bulletin Board in room 3000 O 1 3096 5 3003         Social Bulletin Board in room 3003 O 1 3096 5 3018         Social Bulletin Board in room 3018 O 1 3096 5 3022         Social Bulletin Board in room 3022 O 1 3096 5 3028         Social Bulletin Board in room 3028 * "S" must appear after all commands for a particular zone S`

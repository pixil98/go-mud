4\. Mobile (Monster) Files
--------------------------

4.1 The Format of a Mobile
--------------------------

The format of a mobile is:

> `* * *  #<virtual number> <alias list>~ <short description>~ <long description> ~ <detailed description> ~ <action bitvector> <affection bitvector> <alignment> <type flag> {type-specific information; see below for details}  * * *`

The format of mobiles varies depending on the Type Flag. See below for documentation of the formats of the various types.

**Virtual Number**

This number is critical; it is the identity of the mobile within the game. It is the number that will be used to reference the mobile from zone files and is the number used to \`\`load'' mobiles from within the game. The virtual numbers must appear in increasing order in the mob file.

**Alias List**

The list of keywords, separated by spaces, that can be used by players to identify the mobile. The mobile can only be identified using the keywords that appear in its alias list; it cannot be identified by a word that appears only in its name. Great care should be taken to ensure that the spellings of names and aliases match. Fill words such as \`\`the,'' \`\`a,'' and \`\`an'' should not appear in the Alias List.

**Short Description**

The description of the mobile used by the MUD when the mobile takes some action. For example, a short description of \`\`The Beastly Fido'' would result in messages such as \`\`The Beastly Fido leaves south.'' and \`\`The Beastly Fido hits you hard.'' The Short Description should never end with a punctuation mark because it will be inserted into the middle of sentences such as those above.

**Long Description**

The description displayed when a mobile is in its default position; for example, \`\`The Beastly Fido is here, searching through garbage for food.'' When the mobile is in a position other than its default position, such as sleeping or incapacitated, the short description is used instead; for example, \`\`The Beastly Fido is lying here, incapacitated.'' Unlike the Short Description, the Long Description should end with appropriate punctuation.

**Detailed Description**

The description displayed for a mobile when a player looks at the mobile by typing \`\`look at <mobile>.''

**Action Bitvector**

A bitvector (see section [Using Bitvectors](building-2.html#bitvectors)) with the following values:

> `* * *  1      a  SPEC           This flag must be set on mobiles which have                          special procedures written in C.  In addition to                          setting this bit, the specproc must be assigned in                          spec_assign.c, and the specproc itself must (of                          course) must be written.  See the section on                          Special Procedures in the file coding.doc for                           more information.  2      b  SENTINEL       Mobiles wander around randomly by default; this                          bit should be set for mobiles which are to remain                          stationary.  4      c  SCAVENGER      The mob should pick up valuables it finds on the                          ground.  More expensive items will be taken first.  8      d  ISNPC          Reserved for internal use.  Do not set.  16     e  AWARE          Set for mobs which cannot be backstabbed.                           Replaces the ACT_NICE_THIEF bit from Diku Gamma.  32     f  AGGRESSIVE     Mob will hit all players in the room it can see.                           See also the WIMPY bit.  64     g  STAY_ZONE      Mob will not wander out of its own zone -- good                          for keeping your mobs as only part of your own                          area.  128    h  WIMPY          Mob will flee when being attacked if it has less                          than 20% of its hit points.  If the WIMPY bit is                          set in conjunction with any of the forms of the                          AGGRESSIVE bit, the mob will only attack mobs that                          are unconscious (sleeping or incapacitated).  256    i  AGGR_EVIL      Mob will attack players that are evil-aligned.  512    j  AGGR_GOOD      Mob will attack players that are good-aligned.  1024   k  AGGR_NEUTRAL   Mob will attack players that are neutrally aligned.  2048   l  MEMORY         Mob will remember the players that initiate                          attacks on it, and initiate an attack on that                          player if it ever runs into him again.  4096   m  HELPER         The mob will attack any player it sees in the room                          that is fighting with a mobile in the room.                           Useful for groups of mobiles that travel together;                          i.e. three snakes in a pit, to force players to                          fight all three simultaneously instead of picking                          off one at a time.  8192   n  NOCHARM        Mob cannot be charmed.  16384  o  NOSUMMON       Mob cannot be summoned.  32768  p  NOSLEEP        Sleep spell cannot be cast on mob.  65536  q  NOBASH         Large mobs such as trees that cannot be bashed.  131072 r  NOBLIND        Mob cannot be blinded.  * * *`

**Affection Bitvector**

A bitvector (see section [Using Bitvectors](building-2.html#bitvectors)) with the following values:

> `* * *  1       a BLIND          Mob is blind. 2       b INVISIBLE      Mob is invisible. 4       c DETECT_ALIGN   Mob is sensitive to the alignment of others. 8       d DETECT_INVIS   Mob can see invisible characters and objects. 16      e DETECT_MAGIC   Mob is sensitive to magical presence. 32      f SENSE_LIFE     Mob can sense hidden life. 64      g WATERWALK      Mob can traverse unswimmable water sectors. 128     h SANCTUARY      Mob is protected by sanctuary (half damage). 256     i GROUP          Reserved for internal use.  Do not set. 512     j CURSE          Mob is cursed. 1024    k INFRAVISION    Mob can see in dark. 2048    l POISON         Reserved for internal use.  Do not set. 4096    m PROTECT_EVIL   Mob is protected from evil characters. 8192    n PROTECT_GOOD   Mob is protected from good characters. 16384   o SLEEP          Reserved for internal use.  Do not set. 32768   p NOTRACK        Mob cannot be tracked. 65536   q UNUSED16       Unused (room for future expansion). 131072  r UNUSED17       Unused (room for future expansion). 262144  s SNEAK          Mob can move quietly (room not informed). 524288  t HIDE           Mob is hidden (only visible with sense life). 1048576 u UNUSED20       Unused (room for future expansion). 2097152 v CHARM          Reserved for internal use.  Do not set.  * * *`

**Alignment**

A number from -1000 to 1000 representing the mob's initial alignment.

>      `-1000...-350   Evil       -349...349    Neutral        350...1000   Good`

**Type Flag**

This flag is a single letter which indicates what type of mobile is currently being defined, and controls what information CircleMUD expects to find next (i.e., in the file from the current point to the end of the current mobile).

Standard CircleMUD 3.0 supports two types of mobiles: S (for Simple), and E (for Enhanced). Type C (Complex) mobiles was part of the original DikuMUD Gamma and part of CircleMUD until version 3.0, but are no longer supported by CircleMUD v3.0 and above.

Check with your local implementor to see if there are any additional types supported on your particular MUD.

4.2 Type S Mobiles
------------------

For type S mobs, the type-specific information should be in the following format:

> `* * *  <level> <thac0> <armor class> <max hit points> <bare hand damage> <gold> <experience points> <load position> <default position> <sex>  * * *`

**Level**

The level of the monster, from 1 to 30.

**THAC0**

\`\`To Hit Armor Class 0'' -- a measure of the ability of the monster to penetrate armor and cause damage, ranging from 0 to 20. Lower numbers mean the monster is more likely to penetrate armor. The formal definition of THAC0 is the minimum roll required on a 20-sided die required to hit an opponent of equivalent Armor Class 0.

**Armor Class**

The ability of the monster to avoid damage. Range is from -10 to 10, with lower values indicating better armor. Roughly, the scale is:

>      `AC  10    Naked person      AC   0    Very heavily armored person (full plate mail)      AC -10    Armored Battle Tank (hopefully impossible for players)`

Note on THAC0 and Armor Class (AC): When an attacker is trying to hit a victim, the attacker's THAC0 and the victim's AC, plus a random roll of the dice, determines whether or not the attacker can hit the victim. (If a hit occurs, a different formula determines how much damage is done.) An attacker with a low THAC0 is theoretically just as likely to hit a victim with a low AC as an attacker with a high THAC0 is to hit a victim with a high AC. Lower attacker THAC0's and higher victim AC's favor the attacker; higher attacker THAC0's and lower victim AC's favor the victim.

**Max Hit Points**

The maximum number of hit points the mobile is given, which must be given in the form \`\`xdy+z'' where x, y, and z are integers. For example, `4d6+10` would mean sum 4 rolls of a 6 sided die and add 10 to the result. Each individual instance of a mob will have the same max number of hit points from the time it's born to the time it dies; the dice will only be rolled once when a particular instance of the mob is created. In other words, a particular copy of a mob will always have the same number of max hit points during its life, but different copies of the same mob may have different numbers of max hit points.

Note that all three numbers, the \`\``d`'' and the \`\``+`'' must always appear, even if some of the numbers are 0. For example, if you want every copy of a mob to always have exactly 100 hit points, write `0d0+100`.

**Bare Hand Damage (BHD)**

The amount of damage the mob can do per round when not armed with a weapon. Also specified as \`\`xdy+z'' and subject to the same formatting rules as Max Hit Points. However, unlike Max Hit Points, the dice are rolled once per round of violence; the BHD of a mob will vary from round to round, within the limits you set.

For BHD, xdy specifies the dice rolls and z is the strength bonus added both to BHD and weapon-inflicted damage. For example, a monster with a BHD of 1d4+10 will do between 11 and 14 hitpoints each round without a weapon. If the monster picks up and wields a tiny stick which gives 1d2 damage, then the monster will do 1d2 + 10 points of damage per round with the stick.

**Gold**

The number of gold coins the mobile is born with.

**Experience**

The number of experience points the mobile is born with.

**Load Position**

The position the mobile is in when born, which should be one of the following numbers:

> `0   POSITION_DEAD       Reserved for internal use.  Do not set. 1   POSITION_MORTALLYW  Reserved for internal use.  Do not set. 2   POSITION_INCAP      Reserved for internal use.  Do not set. 3   POSITION_STUNNED    Reserved for internal use.  Do not set. 4   POSITION_SLEEPING   The monster is sleeping. 5   POSITION_RESTING    The monster is resting. 6   POSITION_SITTING    The monster is sitting. 7   POSITION_FIGHTING   Reserved for internal use.  Do not set. 8   POSITION_STANDING   The monster is standing.`

**Default Position**

The position to which monsters will return after a fight, which should be one of the same numbers as given above for Load Position. In addition, the Default Position defines when the mob's long description is displayed (see \`\`Long Description'' above).

**Sex**

One of the following:

>      `0    Neutral (it/its)      1    Male (he/his)      2    Female (she/her)`

4.3 Type S Mobile Example
-------------------------

> `#3062 fido dog~ the beastly fido~ A beastly fido is mucking through the garbage looking for food here. ~ The fido is a small dog that has a foul smell and pieces of rotted meat hanging around his teeth. ~ afghq p -200 S 0 20 10 1d6+4 1d4+0 0 25 8 8 1`

This is mobile vnum 3062. The Fido's action bitvector indicates that it has a special procedure (bit a), is aggressive (bit f), stays in its own zone (bit g), is wimpy (bit h), and cannot be bashed (bit q). Also, the Fido cannot be tracked (affection bit p), and has an initial alignment of -200.

After the `S` flag we see that the Fido is level 0, has a THAC0 of 20, an Armor Class of 10, 1d6+4 hit points (a random value from 5 to 10), and will do 1d4 hit points of bare hand damage per round. The Fido has 0 gold and 25 experience points, has a load position and default position of STANDING, and is male.

4.4 Type E Mobiles
------------------

Type E mobiles are specific to Circle 3.0 and are designed to provide an easy way for MUD implementors to extend the mobile format to fit their own needs. A type E mobile is an extension of type S mobiles; a type E mobile is a type S mobile with extra data at the end. After the last line normally found in type S mobs (the one ending with the mob's sex), type E mobiles end with a section called the Enhanced section. This section consists of zero or more enhanced mobile specifications (or _E-specs_), one per line. Each E-spec consists of a keyword followed by a colon (\`\`:'') and a value. The valid keywords are listed below. The literal letter `E` must then come after all E-specs to signal the end of the mob.

The format of an E mobile is as follows:

> `* * *  <level> <thac0> <armor class> <max hit points> <bare hand damage> <gold> <experience points> <load position> <default position> <sex> {E-spec list} E  * * *`

4.5 Type E Mobile Example
-------------------------

Let's say that you wanted to create an enhanced Fido like the one in the previous example, but one that has a bare-hand attack type of 4 so that the Fido bites players instead of hitting them. Let's say you also wanted to give this Fido the a strength of 18. You might write:

> `#3062 fido dog~ the beastly fido~ A beastly fido is mucking through the garbage looking for food here. ~ The fido is a small dog that has a foul smell and pieces of rotted meat hanging around his teeth. ~ afghq p -200 E 0 20 10 1d6+4 1d4+0 0 25 8 8 1 BareHandAttack: 4 Str: 18 E`

In the above example, the two E-specs used were BareHandAttack and Str. Any number of the E-specs can be used in an Enhanced section and they may appear in any order. The format is simple: the E-spec keyword, followed by a colon, followed by a value. Note that unlike type S mobiles, type E mobiles require a terminator at the end of the record (the letter `E`).

4.6 E-Spec Keywords Valid in CircleMUD 3.00
-------------------------------------------

The only keywords supported under Circle 3.00 are BareHandAttack, Str, StrAdd, Int, Wis, Dex, Con, and Cha. However, the E-Specs have been designed such that new ones are quite easy to add; check with your local implementor to see if your particular MUD has added any additional E-Specs. Circle 3.10's Enhanced section will have considerably more features available such as the ability to individually set mobs' skill proficiencies.

**BareHandAttack**

This controls the description of violence given during battles, in messages such as \`\`The Beastly fido bites you very hard.'' BareHandAttack should be one of the following numbers:

>      `0    hit/hits      1    sting/stings      2    whip/whips      3    slash/slashes      4    bite/bites      5    bludgeon/bludgeons      6    crush/crushes      7    pound/pounds      8    claw/claws      9    maul/mauls      10   thrash/thrashes      11   pierce/pierces      12   blast/blasts      13   punch/punches      14   stab/stabs`

Messages given when attackers miss or kill their victims are taken from the file `lib/misc/messages`. The attack type number for weapons is 300 plus the number listed in the table above, so to modify the message given to players when they are mauled, attack type number 309 in `lib/misc/messages` should be changed. Note that adding new attack types requires code changes and _cannot_ be accomplished simply by adding new messages to `lib/misc/messages` (see the [CircleMUD Coding Manual](http://www.circlemud.org/~jelson/circle/cdp/coding) for more information).

**Str, StrAdd, Int, Wis, Dex, Con, Cha**

The mobile's Strength, Strength Add, Intelligence, Wisdom, Dexterity, Constitution and Charisma, respectively. These values should be between 3 and 18.

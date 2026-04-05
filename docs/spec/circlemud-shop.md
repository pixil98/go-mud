7\. Shop Files
--------------

CircleMUD v3.0 now has a new shop file format. Since the old format is still supported, both formats will be documented. If you'd like to convert shop files in the old format to that of the new format, compile and run the utility shopconv. 3.0 shops must have a special marker (described below) to tell the server that the file is in the new format.

7.1 CircleMUD v3.0 Shop Format
------------------------------

The overall format of a v3.0 Shop File is:

> `* * *  CircleMUD v3.0 Shop File~ <Shop 1> <Shop 2> . . . <Shop n> $~  * * *`

3.0 shop files start with the literal line \`\``CircleMUD v3.0 Shop File~`'', followed by any number of shop definitions, and terminated by $~. The format of a shop definition is:

> `* * *  #<Shop Number>~ <Item Vnum 1> <Item Vnum 2> <Item Vnum 3>   .   .   . <Item Vnum n> -1 <Profit when selling> <Profit when buying> <Buy Type 1>  [Buy Namelist 1] <Buy Type 2>  [Buy Namelist 1] <Buy Type 3>  [Buy Namelist 1]   .   .   . <Buy Type n>  [Buy Namelist n] -1 <Message when item to buy does not exist>~ <Message when item to sell does not exist>~ <Message when shop does not buy offered item>~ <Message when shop can't afford item>~ <Message when player can't afford item>~ <Message when successfully buying an item>~ <Message when successfully selling an item>~ <Temper> <Shop Bitvector> <Shop Keeper Mobile Number> <With Who Bitvector> <Shop Room 1> <Shop Room 2> <Shop Room 3>    .    .    . <Shop Room n> -1 <Time when open start 1> <Time when open end 1> <Time when open start 2> <Time when open end 2>  * * *`

**Shop Number**

A unique number for the shop (used only for display purposes).

**Item Vnum 1...Item Vnum n**

An arbitrarily long list of the virtual numbers of objects that the shop produces (i.e., items which will always be available, no matter how many are bought). The list must be terminated with -1.

**Profit When Selling**

The price of an object when a shopkeeper sells it is the object's value times Profit When Selling. This is a floating point value. It should be >= 1.0.

**Profit When Buying**

The amount of money a shopkeeper will offer when buying an object is the object's value times Profit When Buying. This is a floating point value. It should be <= 1.0.

**Buy Types and Buy Namelists**

These lines control what types of items that the shop will buy. There can be an arbitrarily long list of buy types terminated by -1. The first argument, called \`\`Buy Type'' is the Type Flag of items the shop will buy (see \`\`Type Flag'' under \`\`Format of an Object'' in this document). Numerical and English forms are both valid (5 or WEAPON, 9 or ARMOR, etc.).

The second (optional) argument is called a Buy Namelist and allows you to provide optional keywords to define specific keywords that must be present on the objects for the shopkeeper to buy or sell it. For further details on these expressions, see the section \`\`Item Name Lists'' below.

**Message when item to buy does not exist**

The message given to a player if he tries to buy an item that the shopkeeper does not have in his inventory.

**Message when item to sell does not exist**

The message given to a player if he tries to sell an item that the player does not have in his inventory.

**Message when shop does not buy offered item**

The message given to a player if he tries to sell an item that the shopkeeper does not want to buy (controlled by the Buy Types and Buy Namelists.)

**Message when shop can't afford item**

The message given to a player if he tries to sell an item to a shop, but the shopkeeper does not have enough money to buy it.

**Message when player can't afford item**

The message given to a player if he tries to buy an item from a shop but doesn't have enough money.

**Message when successfully buying an item**

The message given to a player when he successfully buys an item from a shop. The expression `%d` can be used in place of the cost of the item (e.g., `That'll cost you %d coins, thanks for your business!`

**Message when successfully selling an item**

The message given to a player when he successfully sells an item to a shop. The expression `%d` can be used in place of the cost of the item as above.

**Temper**

When player can't afford an item, the shopkeeper tells them they can't afford the item and then can perform an additional action (-1, 0, or 1):

**\-1**

No action other than the message.

**0**

The shopkeeper pukes on the player.

**1**

The shopkeeper smokes his joint.

Further actions can be added by your local coder (e.g., attacking a player, stealing his money, etc.)

**Shop Bitvector**

A bitvector (see section [Using Bitvectors](building-2.html#bitvectors)) with the following values:

> `* * *  1    a    WILL_START_FIGHT    Players can to try to kill shopkeeper. 2    b    WILL_BANK_MONEY     Shopkeeper will put money over 15000                               coins in the bank.  * * *`

A brief note: Shopkeepers should be hard (if even possible) to kill. The benefits players can receive from killing them is enough to unbalance most non monty-haul campaigns.

**Shop Keeper Mobile Number**

Virtual number of the shopkeeper mobile.

**With Who Bitvector**

A bitvector (see section [Using Bitvectors](building-2.html#bitvectors)) used to designate certain alignments or classes that the shop will not trade with, with the following values:

> `* * *  1     a   NOGOOD         Don't trade with positively-aligned players. 2     b   NOEVIL         Don't trade with evilly-aligned players. 4     c   NONEUTRAL      Don't trade with neutrally-aligned players. 8     d   NOMAGIC_USER   Don't trade with the Mage class. 16    e   NOCLERIC       Don't trade with the Cleric class. 32    f   NOTHIEF        Don't trade with the Thief class. 64    g   NOWARRIOR      Don't trade with the Warrior class.  * * *`

**Shop Room 1...Shop Room n**

The virtual numbers the mobile must be in for the shop to be effective. (So trans'ed shopkeepers can't sell in the desert). The list can be arbitrarily long but must be terminated by a -1.

**Times when open**

The times (in MUD-hours) between which the shop is open. Two sets of Open/Close pairs are allowed so that the shop can be open twice a day (for example, once in the morning and once at night). To have a shop which is always open, these four values should be

> `0 28 0 0`

7.2 Item Name Lists for 3.0 Shops
---------------------------------

Name lists are formed by boolean expressions. The following operators are available:

>           `',^ = Not      *, & = And     +, | = Or`

The precedence is Parenthesis, Not, And, Or. Take the following line for an example:

`WEAPON [sword & long|short | warhammer | ˆgolden & bow] & magic`

This shop will buy the following items of type WEAPON:

1.  sword long magic
2.  short magic (the first & is done before the first | )
3.  warhammer magic
4.  ˆgolden bow magic

Note that the ˆ in front of golden affects ONLY golden, and nothing else in the listing. Basically, the above expression could be written in English as:

`[(sword and long) or short or warhammer or (not golden and bow)] and magic`

If you want the shop to only buy \`\`short magic'' only if they were also swords, you could change the expression to:

WEAPON \[sword & (long|short) | warhammer | ^golden & bow\] & magic
                ^-Changes--^ 

You can also include object extra flags (listed in the section \`\`Format of an Object'' above). The previous example used \`\`magic'' as a keyword that had to be on the object. If we wanted to make it so that the MAGIC flag had to be set on the item, we would change \`\`magic'' to \`\`MAGIC.'' Similar changes could be made to add other flags such as \`\`HUM'' or \`\`GLOW.'' It should be noted that these expressions are case sensitive and that all keywords should appear in lower-case, while the flag names should be in all caps.

7.3 The DikuMUD Gamma and CircleMUD 2.20 Shop Format
----------------------------------------------------

This format is obsolete but is presented because it is still supported by CircleMUD 3.0. In most cases, it is easier to simply use the \`\`shopconv'' utility shipped with Circle to convert older shop files to the new format.

#num~
     Shop Number (Used only for display purposes)

num1
num2
num3
num4
num5
     Virtual numbers of the objects that the shop produces.  -1's should be
inserted in unused slots.

Profit when selling
     The object value is multiplied by this value when sold.  This is a
floating point value.  Must be >= 1.0

Profit when buying
     The object value is multiplied by this value when bought.  This is a
floating point value.  Must be <= 1.0

num1
num2
num3
num4
num5
     These five numbers are the item-types traded with by the shop (i.e.
valid Type Flags of objects that the shopkeeper will buy).

Message When Item to buy is non existing~
Message When item trying to sell is non existing~
Message When wrong item-type sold~
Message when shop can't afford item~
Message when player can't afford item~
Message when buying an item~
     Price is represented by %d.
Message when selling an item~
     Price is represented by %d.

Temper
     When player can't afford an item, the shopkeeper tells them they can't
afford the item and then:
     0 - The shopkeeper pukes on the player.
     1 - The shopkeeper smokes his joint.
     other - No action besides message above.

Shop Bitvector
     A bitvector (see section "Using Bitvectors" above) with the following values:

1    a    WILL\_START\_FIGHT    Players can to try to kill this shopkeeper.
2    b    WILL\_BANK\_MONEY     Shopkeeper will put money over 15000 coins
                              in the bank.

     A brief note:  Shopkeepers should be hard (if even possible) to kill. 
The benefits players can receive from killing them is enough to unbalance
most non monty-haul campaigns.

Shop Keeper Mobile Number
     Virtual number of the shopkeeper mobile.

With Who Bitvector
     A bitvector (see section "Using Bitvectors" above) used to designate certain
alignments or classes that the shop will not trade with, with the following
values:

1     a   NOGOOD         Keeper won't trade with positively-aligned players.
2     b   NOEVIL         Keeper won't trade with evilly-aligned players.
4     c   NONEUTRAL      Keeper won't trade with neutrally-aligned players.
8     d   NOMAGIC\_USER   Keeper won't trade with the Mage class.
16    e   NOCLERIC       Keeper won't trade with the Cleric class.
32    f   NOTHIEF        Keeper won't trade with the Thief class.
64    g   NOWARRIOR      Keeper won't trade with the Warrior class.

Shop Room Number
     The virtual number the mobile must be in for the shop to be effective.
(So trans'ed shopkeepers can't sell in the desert).

Time when open start 1
Time when open end 1
     The hours between which the shop is open.

Time when open start 2
Time when open end 2
     The hours between which the shop is open.

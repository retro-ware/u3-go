# Game data: PC and Mac

`internal/res` reads either release into one `Resources`, numbered like the
LairWare Mac resources (BASERES = 400).

## PC release (.ULT files)

| File | Mac resource | Format |
| --- | --- | --- |
| BRITISH, EXODUS, LCB, MOON, YEW, MONTOR-E, MONTOR-W, GREY, DAWN, DEVIL, FAWN, DEATH, SOSARIA, AMBROSIA .ULT (4648 bytes) | MAPS/TLKS/MONS 400-411, 420, 421 | 64x64 tiles, 384-byte talk block, 168-byte monster table |
| M, FIRE, TIME, P, PERINIAN, MINE, DARDIN .ULT (2192) | MAPS/TLKS 412-418 | eight 16x16 levels, 144-byte talk block |
| CNFLCT-A,B,C,F,G,M,Q,R,S.ULT (176) | CONS 400-408 | Mac layout, tile numbers halved |
| SHRINE, FOUNTAIN, TIME, BRAND .IMG (121) | CONS 410-413 | 11x11 tiles, halved |
| ROSTER.ULT (1280) | ROST 500 | same layout |

- Map tiles are numbered as on the Mac; the Mac adds a leading edge-length
  byte (64) and, for Sosaria, four whirlpool bytes the PC keeps elsewhere.
- Talk: a table of 16-bit offsets (the first is the table's own length),
  then NUL-terminated mixed-case ASCII with `\n` breaks and `` ` `` for an
  opening quote. The Mac's is upper case, high bit set, 0xFF breaks, each
  line led by a NUL.
- Monster table: the first 168 bytes of the Mac's 256 (the rest are zero).
- Arenas: tiles, the tiles under monsters (offset 144) and under members
  (168) are half the Mac's numbers.
- Not in data files on the PC (taken from the Mac in `pctables.go`): the
  MISC tables, the default party record, map names, the whirlpool, and all
  Strings plists. PARTY.ULT and ROSTER.ULT ship blank (all zero); the Mac's
  default roster holds pre-made characters, the PC's is empty.
- Not read: MOVES.ULT, DEMO.ULT, DUNGEON.DAT, NAME.DAT, ANIMATE.DAT, *.IBM.

## Where the games differ

- Yew (404): LairWare added Shawn ("I GOO FOR LIV!"), one map tile and one
  monster slot.
- Ambrosia (421): LairWare added Beastie and Reesie (two map tiles, monster
  slots, and its own talk; the PC's Ambrosia talk is a copy of LCB's).
- Sosaria (420): the PC's monster table has bytes in the sixth row
  (`05 3a ... 0c 04`) where the Mac's is zero.
- Talk wording: the Mac fixes spelling and punctuation (PANNELS -> PANELS,
  COLOSAL -> COLLASAL, "THE FOOD STINKS!" -> "SUCKS!!", added commas) and
  restores lines the PC lost to its offset table overlapping the text
  (Exodus, Yew, Death Gulch, Grey, Fawn and the Montors start mid-word on
  the PC; Clues repeats "DON'T DRINK!" on the Mac).
- Talk blocks on both carry leftover bytes after the last line; they are
  not compared.

## LairWare 2.0 (2007 release) vs the repository's sources

The 2.0 app keeps `Contents/Resources/Ultima III.rsrc` and
`Contents/Resources/Strings/*.plist` where the repository has
`English.lproj/MainResources.rsrc` and `English.lproj/Strings`; `LoadMac`
accepts either layout.

Its data matches the repository's except: the default roster (ROST 500)
differs, Strings has no MoreMessages.plist (filled from `pctables.go`), and
Messages/TilesVoices differ in a few quotes and line wording.

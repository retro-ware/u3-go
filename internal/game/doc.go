// Package game is Ultima III's game logic, UI-agnostic: rules, combat,
// dungeons, spells, shops, talk, the quest journal and saving. It is a port
// of the TypeScript engine in synthwave-pixel's ultima3 by Leon McNeill
// (MIT licensed), itself after LairWare's Macintosh C source.
//
// # Data
//
// Game data arrives as a *res.Resources. All mutable state lives in a
// *World; the party and roster are byte views (Party, PlayerRecord) over the
// records the save file stores.
//
// # Blocking IO instead of async
//
// The TypeScript port made every wait a Promise and every routine that might
// wait async. Here the game runs on its own goroutine and calls the IO
// interface (io.go) synchronously: a method that waits for the player (a
// key, a menu choice, a line of text, a pause) simply blocks until the UI
// answers, typically by reading from a channel its event loop feeds. Each
// `await f()` of the TypeScript is a plain `f()` call, so the code keeps the
// original's shape line for line. Values the TypeScript returned as null
// come back as "" (keys, menu answers), 0 (members) or nil (slices).
//
// The UI must not touch the World while the game goroutine is running
// except from inside IO calls (when the game is blocked in it), or under its
// own lock around them; drawing from a snapshot taken in RedrawMap,
// UpdateStats and the like is the simple way.
//
// # Randomness
//
// Random is the TypeScript port's mulberry32 with the same draw order, so a
// seeded world plays out exactly as the TypeScript tests expect.
package game

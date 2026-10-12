// Package domain holds the business rules for crackapack: booster generation,
// collection management, the double-entry ledger, coin economy and NPC simulation.
//
// The domain imports no infrastructure. Every dependency — the clock, randomness,
// persistence — arrives through interfaces the domain declares. Adapters implement
// those interfaces and depend on the domain, never the reverse.
package domain

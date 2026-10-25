package domain

import "fmt"

// colourlessPool is the key of the names given to an NPC that has no colour.
const colourlessPool = "C"

// npcNamePools returns the planeswalkers and the legendary creatures an NPC is named after, by the
// Magic colour of the character, and the colourless ones under colourlessPool.
func npcNamePools() map[string][]string {
	return map[string][]string{
		"W": {
			"Ajani Goldmane", "Elspeth Tirel", "Gideon Jura", "Basri Ket", "Lyra Dawnbringer",
			"Thalia, Guardian of Thraben", "Akroma, Angel of Wrath", "Brimaz, King of Oreskos",
			"Linvala, Keeper of Silence", "Mikaeus, the Lunarch", "Isamaru, Hound of Konda",
			"Sigarda, Host of Herons", "Kytheon, Hero of Akros", "Archangel Avacyn", "Mangara of Corondor",
		},
		"U": {
			"Jace Beleren", "Jace, the Mind Sculptor", "Tamiyo, the Moon Sage", "Teferi, Master of Time",
			"Tezzeret the Seeker", "Narset, Parter of Veils", "Mu Yanling, Sky Dancer", "Lorthos, the Tidemaker",
			"Talrand, Sky Summoner", "Arcanis the Omnipotent", "Thassa, God of the Sea", "Kopala, Warden of Waves",
			"Vendilion Clique", "Venser, Shaper Savant",
		},
		"B": {
			"Liliana Vess", "Liliana of the Veil", "Ob Nixilis Reignited", "Davriel Cane",
			"Kalitas, Traitor of Ghet", "Gix, Yawgmoth Praetor", "Yawgmoth, Thran Physician",
			"Mikaeus, the Unhallowed", "Tasigur, the Golden Fang", "Demonlord Belzenlok",
			"Ayara, First of Locthwain", "Phage the Untouchable",
		},
		"R": {
			"Chandra Nalaar", "Chandra, Torch of Defiance", "Koth of the Hammer", "Jaya Ballard",
			"Daretti, Scrap Savant", "Tibalt, the Fiend-Blooded", "Krenko, Mob Boss", "Squee, Goblin Nabob",
			"Hazoret the Fervent", "Kiki-Jiki, Mirror Breaker", "Torbran, Thane of Red Fell",
			"Purphoros, God of the Forge",
		},
		"G": {
			"Garruk Wildspeaker", "Garruk Relentless", "Nissa, Worldwaker", "Nissa, Vastwood Seer", "Vivien Reid",
			"Titania, Protector of Argoth", "Multani, Maro-Sorcerer", "Yeva, Nature's Herald",
			"Ezuri, Renegade Leader", "Omnath, Locus of Mana", "Azusa, Lost but Seeking", "Ghalta, Primal Hunger",
		},
		colourlessPool: {
			"Karn Liberated", "Karn, the Great Creator", "Karn, Scion of Urza", "Karn, Silver Golem",
			"Ugin, the Spirit Dragon", "Ugin, the Ineffable", "Ulamog, the Ceaseless Hunger",
			"Ulamog, the Infinite Gyre", "Kozilek, Butcher of Truth", "Kozilek, the Great Distortion",
			"Emrakul, the Aeons Torn", "Emrakul, the Promised End", "Memnarch",
		},
	}
}

// nameNPCs gives each NPC the next unused name of a colour it has: the colours of an NPC take turns
// by its ID, and an NPC with none draws from the colourless pool. A pool that runs out starts over
// with a number after the name, so that every name in the crowd is its own. It draws no random
// number, so that naming leaves the seeded crowd as it was.
func nameNPCs(npcs []*NPC) {
	pools := npcNamePools()
	used := make(map[string]int)
	for _, npc := range npcs {
		pool := colourlessPool
		if len(npc.Colors) > 0 {
			pool = npc.Colors[npc.ID%len(npc.Colors)]
		}
		names := pools[pool]
		turn := used[pool]
		used[pool]++
		npc.Name = names[turn%len(names)]
		if round := turn / len(names); round > 0 {
			npc.Name = fmt.Sprintf("%s %d", npc.Name, round+1)
		}
	}
}

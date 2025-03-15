# A Go Template-Based SQL Builder and Struct Mapper

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)
[![codecov](https://codecov.io/github/wroge/sqlt/graph/badge.svg?token=GDAWVVKGMR)](https://codecov.io/github/wroge/sqlt)

```go
import "github.com/wroge/sqlt"
```

`sqlt` uses Go’s template engine to create a flexible, powerful, and type-safe SQL builder and struct mapper.  

- Vertical Slice Architecture [Example](https://github.com/wroge/vertical-slice-architecture)

# Example

<details>
  <summary>pokemon_data_pokeapi.csv</summary>

  - Dataset from [kaggle](https://www.kaggle.com/datasets/mohitbansal31s/pokemon-dataset)

```csv
Name,Pokedex Number,Type1,Type2,Classification,Height (m),Weight (kg),Abilities,Generation,Legendary Status
Bulbasaur,1,Grass,Poison,Seed Pokémon,0.7,6.9,"Overgrow, Chlorophyll",1,No
Ivysaur,2,Grass,Poison,Seed Pokémon,1.0,13.0,"Overgrow, Chlorophyll",1,No
Venusaur,3,Grass,Poison,Seed Pokémon,2.0,100.0,"Overgrow, Chlorophyll",1,No
Charmander,4,Fire,,Lizard Pokémon,0.6,8.5,"Blaze, Solar-power",1,No
Charmeleon,5,Fire,,Flame Pokémon,1.1,19.0,"Blaze, Solar-power",1,No
Charizard,6,Fire,Flying,Flame Pokémon,1.7,90.5,"Blaze, Solar-power",1,No
Squirtle,7,Water,,Tiny Turtle Pokémon,0.5,9.0,"Torrent, Rain-dish",1,No
Wartortle,8,Water,,Turtle Pokémon,1.0,22.5,"Torrent, Rain-dish",1,No
Blastoise,9,Water,,Shellfish Pokémon,1.6,85.5,"Torrent, Rain-dish",1,No
Caterpie,10,Bug,,Worm Pokémon,0.3,2.9,"Shield-dust, Run-away",1,No
Metapod,11,Bug,,Cocoon Pokémon,0.7,9.9,Shed-skin,1,No
Butterfree,12,Bug,Flying,Butterfly Pokémon,1.1,32.0,"Compound-eyes, Tinted-lens",1,No
Weedle,13,Bug,Poison,Hairy Bug Pokémon,0.3,3.2,"Shield-dust, Run-away",1,No
Kakuna,14,Bug,Poison,Cocoon Pokémon,0.6,10.0,Shed-skin,1,No
Beedrill,15,Bug,Poison,Poison Bee Pokémon,1.0,29.5,"Swarm, Sniper",1,No
Pidgey,16,Normal,Flying,Tiny Bird Pokémon,0.3,1.8,"Keen-eye, Tangled-feet, Big-pecks",1,No
Pidgeotto,17,Normal,Flying,Bird Pokémon,1.1,30.0,"Keen-eye, Tangled-feet, Big-pecks",1,No
Pidgeot,18,Normal,Flying,Bird Pokémon,1.5,39.5,"Keen-eye, Tangled-feet, Big-pecks",1,No
Rattata,19,Normal,,Mouse Pokémon,0.3,3.5,"Run-away, Guts, Hustle",1,No
Raticate,20,Normal,,Mouse Pokémon,0.7,18.5,"Run-away, Guts, Hustle",1,No
Spearow,21,Normal,Flying,Tiny Bird Pokémon,0.3,2.0,"Keen-eye, Sniper",1,No
Fearow,22,Normal,Flying,Beak Pokémon,1.2,38.0,"Keen-eye, Sniper",1,No
Ekans,23,Poison,,Snake Pokémon,2.0,6.9,"Intimidate, Shed-skin, Unnerve",1,No
Arbok,24,Poison,,Cobra Pokémon,3.5,65.0,"Intimidate, Shed-skin, Unnerve",1,No
Pikachu,25,Electric,,Mouse Pokémon,0.4,6.0,"Static, Lightning-rod",1,No
Raichu,26,Electric,,Mouse Pokémon,0.8,30.0,"Static, Lightning-rod",1,No
Sandshrew,27,Ground,,Mouse Pokémon,0.6,12.0,"Sand-veil, Sand-rush",1,No
Sandslash,28,Ground,,Mouse Pokémon,1.0,29.5,"Sand-veil, Sand-rush",1,No
Nidoran-f,29,Poison,,Poison Pin Pokémon,0.4,7.0,"Poison-point, Rivalry, Hustle",1,No
Nidorina,30,Poison,,Poison Pin Pokémon,0.8,20.0,"Poison-point, Rivalry, Hustle",1,No
Nidoqueen,31,Poison,Ground,Drill Pokémon,1.3,60.0,"Poison-point, Rivalry, Sheer-force",1,No
Nidoran-m,32,Poison,,Poison Pin Pokémon,0.5,9.0,"Poison-point, Rivalry, Hustle",1,No
Nidorino,33,Poison,,Poison Pin Pokémon,0.9,19.5,"Poison-point, Rivalry, Hustle",1,No
Nidoking,34,Poison,Ground,Drill Pokémon,1.4,62.0,"Poison-point, Rivalry, Sheer-force",1,No
Clefairy,35,Fairy,,Fairy Pokémon,0.6,7.5,"Cute-charm, Magic-guard, Friend-guard",1,No
Clefable,36,Fairy,,Fairy Pokémon,1.3,40.0,"Cute-charm, Magic-guard, Unaware",1,No
Vulpix,37,Fire,,Fox Pokémon,0.6,9.9,"Flash-fire, Drought",1,No
Ninetales,38,Fire,,Fox Pokémon,1.1,19.9,"Flash-fire, Drought",1,No
Jigglypuff,39,Normal,Fairy,Balloon Pokémon,0.5,5.5,"Cute-charm, Competitive, Friend-guard",1,No
Wigglytuff,40,Normal,Fairy,Balloon Pokémon,1.0,12.0,"Cute-charm, Competitive, Frisk",1,No
Zubat,41,Poison,Flying,Bat Pokémon,0.8,7.5,"Inner-focus, Infiltrator",1,No
Golbat,42,Poison,Flying,Bat Pokémon,1.6,55.0,"Inner-focus, Infiltrator",1,No
Oddish,43,Grass,Poison,Weed Pokémon,0.5,5.4,"Chlorophyll, Run-away",1,No
Gloom,44,Grass,Poison,Weed Pokémon,0.8,8.6,"Chlorophyll, Stench",1,No
Vileplume,45,Grass,Poison,Flower Pokémon,1.2,18.6,"Chlorophyll, Effect-spore",1,No
Paras,46,Bug,Grass,Mushroom Pokémon,0.3,5.4,"Effect-spore, Dry-skin, Damp",1,No
Parasect,47,Bug,Grass,Mushroom Pokémon,1.0,29.5,"Effect-spore, Dry-skin, Damp",1,No
Venonat,48,Bug,Poison,Insect Pokémon,1.0,30.0,"Compound-eyes, Tinted-lens, Run-away",1,No
Venomoth,49,Bug,Poison,Poison Moth Pokémon,1.5,12.5,"Shield-dust, Tinted-lens, Wonder-skin",1,No
Diglett,50,Ground,,Mole Pokémon,0.2,0.8,"Sand-veil, Arena-trap, Sand-force",1,No
Dugtrio,51,Ground,,Mole Pokémon,0.7,33.3,"Sand-veil, Arena-trap, Sand-force",1,No
Meowth,52,Normal,,Scratch Cat Pokémon,0.4,4.2,"Pickup, Technician, Unnerve",1,No
Persian,53,Normal,,Classy Cat Pokémon,1.0,32.0,"Limber, Technician, Unnerve",1,No
Psyduck,54,Water,,Duck Pokémon,0.8,19.6,"Damp, Cloud-nine, Swift-swim",1,No
Golduck,55,Water,,Duck Pokémon,1.7,76.6,"Damp, Cloud-nine, Swift-swim",1,No
Mankey,56,Fighting,,Pig Monkey Pokémon,0.5,28.0,"Vital-spirit, Anger-point, Defiant",1,No
Primeape,57,Fighting,,Pig Monkey Pokémon,1.0,32.0,"Vital-spirit, Anger-point, Defiant",1,No
Growlithe,58,Fire,,Puppy Pokémon,0.7,19.0,"Intimidate, Flash-fire, Justified",1,No
Arcanine,59,Fire,,Legendary Pokémon,1.9,155.0,"Intimidate, Flash-fire, Justified",1,No
Poliwag,60,Water,,Tadpole Pokémon,0.6,12.4,"Water-absorb, Damp, Swift-swim",1,No
Poliwhirl,61,Water,,Tadpole Pokémon,1.0,20.0,"Water-absorb, Damp, Swift-swim",1,No
Poliwrath,62,Water,Fighting,Tadpole Pokémon,1.3,54.0,"Water-absorb, Damp, Swift-swim",1,No
Abra,63,Psychic,,Psi Pokémon,0.9,19.5,"Synchronize, Inner-focus, Magic-guard",1,No
Kadabra,64,Psychic,,Psi Pokémon,1.3,56.5,"Synchronize, Inner-focus, Magic-guard",1,No
Alakazam,65,Psychic,,Psi Pokémon,1.5,48.0,"Synchronize, Inner-focus, Magic-guard",1,No
Machop,66,Fighting,,Superpower Pokémon,0.8,19.5,"Guts, No-guard, Steadfast",1,No
Machoke,67,Fighting,,Superpower Pokémon,1.5,70.5,"Guts, No-guard, Steadfast",1,No
Machamp,68,Fighting,,Superpower Pokémon,1.6,130.0,"Guts, No-guard, Steadfast",1,No
Bellsprout,69,Grass,Poison,Flower Pokémon,0.7,4.0,"Chlorophyll, Gluttony",1,No
Weepinbell,70,Grass,Poison,Flycatcher Pokémon,1.0,6.4,"Chlorophyll, Gluttony",1,No
Victreebel,71,Grass,Poison,Flycatcher Pokémon,1.7,15.5,"Chlorophyll, Gluttony",1,No
Tentacool,72,Water,Poison,Jellyfish Pokémon,0.9,45.5,"Clear-body, Liquid-ooze, Rain-dish",1,No
Tentacruel,73,Water,Poison,Jellyfish Pokémon,1.6,55.0,"Clear-body, Liquid-ooze, Rain-dish",1,No
Geodude,74,Rock,Ground,Rock Pokémon,0.4,20.0,"Rock-head, Sturdy, Sand-veil",1,No
Graveler,75,Rock,Ground,Rock Pokémon,1.0,105.0,"Rock-head, Sturdy, Sand-veil",1,No
Golem,76,Rock,Ground,Megaton Pokémon,1.4,300.0,"Rock-head, Sturdy, Sand-veil",1,No
Ponyta,77,Fire,,Fire Horse Pokémon,1.0,30.0,"Run-away, Flash-fire, Flame-body",1,No
Rapidash,78,Fire,,Fire Horse Pokémon,1.7,95.0,"Run-away, Flash-fire, Flame-body",1,No
Slowpoke,79,Water,Psychic,Dopey Pokémon,1.2,36.0,"Oblivious, Own-tempo, Regenerator",1,No
Slowbro,80,Water,Psychic,Hermit Crab Pokémon,1.6,78.5,"Oblivious, Own-tempo, Regenerator",1,No
Magnemite,81,Electric,Steel,Magnet Pokémon,0.3,6.0,"Magnet-pull, Sturdy, Analytic",1,No
Magneton,82,Electric,Steel,Magnet Pokémon,1.0,60.0,"Magnet-pull, Sturdy, Analytic",1,No
Farfetchd,83,Normal,Flying,Wild Duck Pokémon,0.8,15.0,"Keen-eye, Inner-focus, Defiant",1,No
Doduo,84,Normal,Flying,Twin Bird Pokémon,1.4,39.2,"Run-away, Early-bird, Tangled-feet",1,No
Dodrio,85,Normal,Flying,Triple Bird Pokémon,1.8,85.2,"Run-away, Early-bird, Tangled-feet",1,No
Seel,86,Water,,Sea Lion Pokémon,1.1,90.0,"Thick-fat, Hydration, Ice-body",1,No
Dewgong,87,Water,Ice,Sea Lion Pokémon,1.7,120.0,"Thick-fat, Hydration, Ice-body",1,No
Grimer,88,Poison,,Sludge Pokémon,0.9,30.0,"Stench, Sticky-hold, Poison-touch",1,No
Muk,89,Poison,,Sludge Pokémon,1.2,30.0,"Stench, Sticky-hold, Poison-touch",1,No
Shellder,90,Water,,Bivalve Pokémon,0.3,4.0,"Shell-armor, Skill-link, Overcoat",1,No
Cloyster,91,Water,Ice,Bivalve Pokémon,1.5,132.5,"Shell-armor, Skill-link, Overcoat",1,No
Gastly,92,Ghost,Poison,Gas Pokémon,1.3,0.1,Levitate,1,No
Haunter,93,Ghost,Poison,Gas Pokémon,1.6,0.1,Levitate,1,No
Gengar,94,Ghost,Poison,Shadow Pokémon,1.5,40.5,Cursed-body,1,No
Onix,95,Rock,Ground,Rock Snake Pokémon,8.8,210.0,"Rock-head, Sturdy, Weak-armor",1,No
Drowzee,96,Psychic,,Hypnosis Pokémon,1.0,32.4,"Insomnia, Forewarn, Inner-focus",1,No
Hypno,97,Psychic,,Hypnosis Pokémon,1.6,75.6,"Insomnia, Forewarn, Inner-focus",1,No
Krabby,98,Water,,River Crab Pokémon,0.4,6.5,"Hyper-cutter, Shell-armor, Sheer-force",1,No
Kingler,99,Water,,Pincer Pokémon,1.3,60.0,"Hyper-cutter, Shell-armor, Sheer-force",1,No
Voltorb,100,Electric,,Ball Pokémon,0.5,10.4,"Soundproof, Static, Aftermath",1,No
Electrode,101,Electric,,Ball Pokémon,1.2,66.6,"Soundproof, Static, Aftermath",1,No
Exeggcute,102,Grass,Psychic,Egg Pokémon,0.4,2.5,"Chlorophyll, Harvest",1,No
Exeggutor,103,Grass,Psychic,Coconut Pokémon,2.0,120.0,"Chlorophyll, Harvest",1,No
Cubone,104,Ground,,Lonely Pokémon,0.4,6.5,"Rock-head, Lightning-rod, Battle-armor",1,No
Marowak,105,Ground,,Bone Keeper Pokémon,1.0,45.0,"Rock-head, Lightning-rod, Battle-armor",1,No
Hitmonlee,106,Fighting,,Kicking Pokémon,1.5,49.8,"Limber, Reckless, Unburden",1,No
Hitmonchan,107,Fighting,,Punching Pokémon,1.4,50.2,"Keen-eye, Iron-fist, Inner-focus",1,No
Lickitung,108,Normal,,Licking Pokémon,1.2,65.5,"Own-tempo, Oblivious, Cloud-nine",1,No
Koffing,109,Poison,,Poison Gas Pokémon,0.6,1.0,"Levitate, Neutralizing-gas, Stench",1,No
Weezing,110,Poison,,Poison Gas Pokémon,1.2,9.5,"Levitate, Neutralizing-gas, Stench",1,No
Rhyhorn,111,Ground,Rock,Spikes Pokémon,1.0,115.0,"Lightning-rod, Rock-head, Reckless",1,No
Rhydon,112,Ground,Rock,Drill Pokémon,1.9,120.0,"Lightning-rod, Rock-head, Reckless",1,No
Chansey,113,Normal,,Egg Pokémon,1.1,34.6,"Natural-cure, Serene-grace, Healer",1,No
Tangela,114,Grass,,Vine Pokémon,1.0,35.0,"Chlorophyll, Leaf-guard, Regenerator",1,No
Kangaskhan,115,Normal,,Parent Pokémon,2.2,80.0,"Early-bird, Scrappy, Inner-focus",1,No
Horsea,116,Water,,Dragon Pokémon,0.4,8.0,"Swift-swim, Sniper, Damp",1,No
Seadra,117,Water,,Dragon Pokémon,1.2,25.0,"Poison-point, Sniper, Damp",1,No
Goldeen,118,Water,,Goldfish Pokémon,0.6,15.0,"Swift-swim, Water-veil, Lightning-rod",1,No
Seaking,119,Water,,Goldfish Pokémon,1.3,39.0,"Swift-swim, Water-veil, Lightning-rod",1,No
Staryu,120,Water,,Star Shape Pokémon,0.8,34.5,"Illuminate, Natural-cure, Analytic",1,No
Starmie,121,Water,Psychic,Mysterious Pokémon,1.1,80.0,"Illuminate, Natural-cure, Analytic",1,No
Mr-mime,122,Psychic,Fairy,Barrier Pokémon,1.3,54.5,"Soundproof, Filter, Technician",1,No
Scyther,123,Bug,Flying,Mantis Pokémon,1.5,56.0,"Swarm, Technician, Steadfast",1,No
Jynx,124,Ice,Psychic,Human Shape Pokémon,1.4,40.6,"Oblivious, Forewarn, Dry-skin",1,No
Electabuzz,125,Electric,,Electric Pokémon,1.1,30.0,"Static, Vital-spirit",1,No
Magmar,126,Fire,,Spitfire Pokémon,1.3,44.5,"Flame-body, Vital-spirit",1,No
Pinsir,127,Bug,,Stag Beetle Pokémon,1.5,55.0,"Hyper-cutter, Mold-breaker, Moxie",1,No
Tauros,128,Normal,,Wild Bull Pokémon,1.4,88.4,"Intimidate, Anger-point, Sheer-force",1,No
Magikarp,129,Water,,Fish Pokémon,0.9,10.0,"Swift-swim, Rattled",1,No
Gyarados,130,Water,Flying,Atrocious Pokémon,6.5,235.0,"Intimidate, Moxie",1,No
Lapras,131,Water,Ice,Transport Pokémon,2.5,220.0,"Water-absorb, Shell-armor, Hydration",1,No
Ditto,132,Normal,,Transform Pokémon,0.3,4.0,"Limber, Imposter",1,No
Eevee,133,Normal,,Evolution Pokémon,0.3,6.5,"Run-away, Adaptability, Anticipation",1,No
Vaporeon,134,Water,,Bubble Jet Pokémon,1.0,29.0,"Water-absorb, Hydration",1,No
Jolteon,135,Electric,,Lightning Pokémon,0.8,24.5,"Volt-absorb, Quick-feet",1,No
Flareon,136,Fire,,Flame Pokémon,0.9,25.0,"Flash-fire, Guts",1,No
Porygon,137,Normal,,Virtual Pokémon,0.8,36.5,"Trace, Download, Analytic",1,No
Omanyte,138,Rock,Water,Spiral Pokémon,0.4,7.5,"Swift-swim, Shell-armor, Weak-armor",1,No
Omastar,139,Rock,Water,Spiral Pokémon,1.0,35.0,"Swift-swim, Shell-armor, Weak-armor",1,No
Kabuto,140,Rock,Water,Shellfish Pokémon,0.5,11.5,"Swift-swim, Battle-armor, Weak-armor",1,No
Kabutops,141,Rock,Water,Shellfish Pokémon,1.3,40.5,"Swift-swim, Battle-armor, Weak-armor",1,No
Aerodactyl,142,Rock,Flying,Fossil Pokémon,1.8,59.0,"Rock-head, Pressure, Unnerve",1,No
Snorlax,143,Normal,,Sleeping Pokémon,2.1,460.0,"Immunity, Thick-fat, Gluttony",1,No
Articuno,144,Ice,Flying,Freeze Pokémon,1.7,55.4,"Pressure, Snow-cloak",1,Yes
Zapdos,145,Electric,Flying,Electric Pokémon,1.6,52.6,"Pressure, Static",1,Yes
Moltres,146,Fire,Flying,Flame Pokémon,2.0,60.0,"Pressure, Flame-body",1,Yes
Dratini,147,Dragon,,Dragon Pokémon,1.8,3.3,"Shed-skin, Marvel-scale",1,No
Dragonair,148,Dragon,,Dragon Pokémon,4.0,16.5,"Shed-skin, Marvel-scale",1,No
Dragonite,149,Dragon,Flying,Dragon Pokémon,2.2,210.0,"Inner-focus, Multiscale",1,No
Mewtwo,150,Psychic,,Genetic Pokémon,2.0,122.0,"Pressure, Unnerve",1,Yes
Mew,151,Psychic,,New Species Pokémon,0.4,4.0,Synchronize,1,Yes
Chikorita,152,Grass,,Leaf Pokémon,0.9,6.4,"Overgrow, Leaf-guard",2,No
Bayleef,153,Grass,,Leaf Pokémon,1.2,15.8,"Overgrow, Leaf-guard",2,No
Meganium,154,Grass,,Herb Pokémon,1.8,100.5,"Overgrow, Leaf-guard",2,No
Cyndaquil,155,Fire,,Fire Mouse Pokémon,0.5,7.9,"Blaze, Flash-fire",2,No
Quilava,156,Fire,,Volcano Pokémon,0.9,19.0,"Blaze, Flash-fire",2,No
Typhlosion,157,Fire,,Volcano Pokémon,1.7,79.5,"Blaze, Flash-fire",2,No
Totodile,158,Water,,Big Jaw Pokémon,0.6,9.5,"Torrent, Sheer-force",2,No
Croconaw,159,Water,,Big Jaw Pokémon,1.1,25.0,"Torrent, Sheer-force",2,No
Feraligatr,160,Water,,Big Jaw Pokémon,2.3,88.8,"Torrent, Sheer-force",2,No
Sentret,161,Normal,,Scout Pokémon,0.8,6.0,"Run-away, Keen-eye, Frisk",2,No
Furret,162,Normal,,Long Body Pokémon,1.8,32.5,"Run-away, Keen-eye, Frisk",2,No
Hoothoot,163,Normal,Flying,Owl Pokémon,0.7,21.2,"Insomnia, Keen-eye, Tinted-lens",2,No
Noctowl,164,Normal,Flying,Owl Pokémon,1.6,40.8,"Insomnia, Keen-eye, Tinted-lens",2,No
Ledyba,165,Bug,Flying,Five Star Pokémon,1.0,10.8,"Swarm, Early-bird, Rattled",2,No
Ledian,166,Bug,Flying,Five Star Pokémon,1.4,35.6,"Swarm, Early-bird, Iron-fist",2,No
Spinarak,167,Bug,Poison,String Spit Pokémon,0.5,8.5,"Swarm, Insomnia, Sniper",2,No
Ariados,168,Bug,Poison,Long Leg Pokémon,1.1,33.5,"Swarm, Insomnia, Sniper",2,No
Crobat,169,Poison,Flying,Bat Pokémon,1.8,75.0,"Inner-focus, Infiltrator",2,No
Chinchou,170,Water,Electric,Angler Pokémon,0.5,12.0,"Volt-absorb, Illuminate, Water-absorb",2,No
Lanturn,171,Water,Electric,Light Pokémon,1.2,22.5,"Volt-absorb, Illuminate, Water-absorb",2,No
Pichu,172,Electric,,Tiny Mouse Pokémon,0.3,2.0,"Static, Lightning-rod",2,No
Cleffa,173,Fairy,,Star Shape Pokémon,0.3,3.0,"Cute-charm, Magic-guard, Friend-guard",2,No
Igglybuff,174,Normal,Fairy,Balloon Pokémon,0.3,1.0,"Cute-charm, Competitive, Friend-guard",2,No
Togepi,175,Fairy,,Spike Ball Pokémon,0.3,1.5,"Hustle, Serene-grace, Super-luck",2,No
Togetic,176,Fairy,Flying,Happiness Pokémon,0.6,3.2,"Hustle, Serene-grace, Super-luck",2,No
Natu,177,Psychic,Flying,Tiny Bird Pokémon,0.2,2.0,"Synchronize, Early-bird, Magic-bounce",2,No
Xatu,178,Psychic,Flying,Mystic Pokémon,1.5,15.0,"Synchronize, Early-bird, Magic-bounce",2,No
Mareep,179,Electric,,Wool Pokémon,0.6,7.8,"Static, Plus",2,No
Flaaffy,180,Electric,,Wool Pokémon,0.8,13.3,"Static, Plus",2,No
Ampharos,181,Electric,,Light Pokémon,1.4,61.5,"Static, Plus",2,No
Bellossom,182,Grass,,Flower Pokémon,0.4,5.8,"Chlorophyll, Healer",2,No
Marill,183,Water,Fairy,Aqua Mouse Pokémon,0.4,8.5,"Thick-fat, Huge-power, Sap-sipper",2,No
Azumarill,184,Water,Fairy,Aqua Rabbit Pokémon,0.8,28.5,"Thick-fat, Huge-power, Sap-sipper",2,No
Sudowoodo,185,Rock,,Imitation Pokémon,1.2,38.0,"Sturdy, Rock-head, Rattled",2,No
Politoed,186,Water,,Frog Pokémon,1.1,33.9,"Water-absorb, Damp, Drizzle",2,No
Hoppip,187,Grass,Flying,Cottonweed Pokémon,0.4,0.5,"Chlorophyll, Leaf-guard, Infiltrator",2,No
Skiploom,188,Grass,Flying,Cottonweed Pokémon,0.6,1.0,"Chlorophyll, Leaf-guard, Infiltrator",2,No
Jumpluff,189,Grass,Flying,Cottonweed Pokémon,0.8,3.0,"Chlorophyll, Leaf-guard, Infiltrator",2,No
Aipom,190,Normal,,Long Tail Pokémon,0.8,11.5,"Run-away, Pickup, Skill-link",2,No
Sunkern,191,Grass,,Seed Pokémon,0.3,1.8,"Chlorophyll, Solar-power, Early-bird",2,No
Sunflora,192,Grass,,Sun Pokémon,0.8,8.5,"Chlorophyll, Solar-power, Early-bird",2,No
Yanma,193,Bug,Flying,Clear Wing Pokémon,1.2,38.0,"Speed-boost, Compound-eyes, Frisk",2,No
Wooper,194,Water,Ground,Water Fish Pokémon,0.4,8.5,"Damp, Water-absorb, Unaware",2,No
Quagsire,195,Water,Ground,Water Fish Pokémon,1.4,75.0,"Damp, Water-absorb, Unaware",2,No
Espeon,196,Psychic,,Sun Pokémon,0.9,26.5,"Synchronize, Magic-bounce",2,No
Umbreon,197,Dark,,Moonlight Pokémon,1.0,27.0,"Synchronize, Inner-focus",2,No
Murkrow,198,Dark,Flying,Darkness Pokémon,0.5,2.1,"Insomnia, Super-luck, Prankster",2,No
Slowking,199,Water,Psychic,Royal Pokémon,2.0,79.5,"Oblivious, Own-tempo, Regenerator",2,No
Misdreavus,200,Ghost,,Screech Pokémon,0.7,1.0,Levitate,2,No
Unown,201,Psychic,,Symbol Pokémon,0.5,5.0,Levitate,2,No
Wobbuffet,202,Psychic,,Patient Pokémon,1.3,28.5,"Shadow-tag, Telepathy",2,No
Girafarig,203,Normal,Psychic,Long Neck Pokémon,1.5,41.5,"Inner-focus, Early-bird, Sap-sipper",2,No
Pineco,204,Bug,,Bagworm Pokémon,0.6,7.2,"Sturdy, Overcoat",2,No
Forretress,205,Bug,Steel,Bagworm Pokémon,1.2,125.8,"Sturdy, Overcoat",2,No
Dunsparce,206,Normal,,Land Snake Pokémon,1.5,14.0,"Serene-grace, Run-away, Rattled",2,No
Gligar,207,Ground,Flying,Fly Scorpion Pokémon,1.1,64.8,"Hyper-cutter, Sand-veil, Immunity",2,No
Steelix,208,Steel,Ground,Iron Snake Pokémon,9.2,400.0,"Rock-head, Sturdy, Sheer-force",2,No
Snubbull,209,Fairy,,Fairy Pokémon,0.6,7.8,"Intimidate, Run-away, Rattled",2,No
Granbull,210,Fairy,,Fairy Pokémon,1.4,48.7,"Intimidate, Quick-feet, Rattled",2,No
Qwilfish,211,Water,Poison,Balloon Pokémon,0.5,3.9,"Poison-point, Swift-swim, Intimidate",2,No
Scizor,212,Bug,Steel,Pincer Pokémon,1.8,118.0,"Swarm, Technician, Light-metal",2,No
Shuckle,213,Bug,Rock,Mold Pokémon,0.6,20.5,"Sturdy, Gluttony, Contrary",2,No
Heracross,214,Bug,Fighting,Single Horn Pokémon,1.5,54.0,"Swarm, Guts, Moxie",2,No
Sneasel,215,Dark,Ice,Sharp Claw Pokémon,0.9,28.0,"Inner-focus, Keen-eye, Pickpocket",2,No
Teddiursa,216,Normal,,Little Bear Pokémon,0.6,8.8,"Pickup, Quick-feet, Honey-gather",2,No
Ursaring,217,Normal,,Hibernator Pokémon,1.8,125.8,"Guts, Quick-feet, Unnerve",2,No
Slugma,218,Fire,,Lava Pokémon,0.7,35.0,"Magma-armor, Flame-body, Weak-armor",2,No
Magcargo,219,Fire,Rock,Lava Pokémon,0.8,55.0,"Magma-armor, Flame-body, Weak-armor",2,No
Swinub,220,Ice,Ground,Pig Pokémon,0.4,6.5,"Oblivious, Snow-cloak, Thick-fat",2,No
Piloswine,221,Ice,Ground,Swine Pokémon,1.1,55.8,"Oblivious, Snow-cloak, Thick-fat",2,No
Corsola,222,Water,Rock,Coral Pokémon,0.6,5.0,"Hustle, Natural-cure, Regenerator",2,No
Remoraid,223,Water,,Jet Pokémon,0.6,12.0,"Hustle, Sniper, Moody",2,No
Octillery,224,Water,,Jet Pokémon,0.9,28.5,"Suction-cups, Sniper, Moody",2,No
Delibird,225,Ice,Flying,Delivery Pokémon,0.9,16.0,"Vital-spirit, Hustle, Insomnia",2,No
Mantine,226,Water,Flying,Kite Pokémon,2.1,220.0,"Swift-swim, Water-absorb, Water-veil",2,No
Skarmory,227,Steel,Flying,Armor Bird Pokémon,1.7,50.5,"Keen-eye, Sturdy, Weak-armor",2,No
Houndour,228,Dark,Fire,Dark Pokémon,0.6,10.8,"Early-bird, Flash-fire, Unnerve",2,No
Houndoom,229,Dark,Fire,Dark Pokémon,1.4,35.0,"Early-bird, Flash-fire, Unnerve",2,No
Kingdra,230,Water,Dragon,Dragon Pokémon,1.8,152.0,"Swift-swim, Sniper, Damp",2,No
Phanpy,231,Ground,,Long Nose Pokémon,0.5,33.5,"Pickup, Sand-veil",2,No
Donphan,232,Ground,,Armor Pokémon,1.1,120.0,"Sturdy, Sand-veil",2,No
Porygon2,233,Normal,,Virtual Pokémon,0.6,32.5,"Trace, Download, Analytic",2,No
Stantler,234,Normal,,Big Horn Pokémon,1.4,71.2,"Intimidate, Frisk, Sap-sipper",2,No
Smeargle,235,Normal,,Painter Pokémon,1.2,58.0,"Own-tempo, Technician, Moody",2,No
Tyrogue,236,Fighting,,Scuffle Pokémon,0.7,21.0,"Guts, Steadfast, Vital-spirit",2,No
Hitmontop,237,Fighting,,Handstand Pokémon,1.4,48.0,"Intimidate, Technician, Steadfast",2,No
Smoochum,238,Ice,Psychic,Kiss Pokémon,0.4,6.0,"Oblivious, Forewarn, Hydration",2,No
Elekid,239,Electric,,Electric Pokémon,0.6,23.5,"Static, Vital-spirit",2,No
Magby,240,Fire,,Live Coal Pokémon,0.7,21.4,"Flame-body, Vital-spirit",2,No
Miltank,241,Normal,,Milk Cow Pokémon,1.2,75.5,"Thick-fat, Scrappy, Sap-sipper",2,No
Blissey,242,Normal,,Happiness Pokémon,1.5,46.8,"Natural-cure, Serene-grace, Healer",2,No
Raikou,243,Electric,,Thunder Pokémon,1.9,178.0,"Pressure, Inner-focus",2,Yes
Entei,244,Fire,,Volcano Pokémon,2.1,198.0,"Pressure, Inner-focus",2,Yes
Suicune,245,Water,,Aurora Pokémon,2.0,187.0,"Pressure, Inner-focus",2,Yes
Larvitar,246,Rock,Ground,Rock Skin Pokémon,0.6,72.0,"Guts, Sand-veil",2,No
Pupitar,247,Rock,Ground,Hard Shell Pokémon,1.2,152.0,Shed-skin,2,No
Tyranitar,248,Rock,Dark,Armor Pokémon,2.0,202.0,"Sand-stream, Unnerve",2,No
Lugia,249,Psychic,Flying,Diving Pokémon,5.2,216.0,"Pressure, Multiscale",2,Yes
Ho-oh,250,Fire,Flying,Rainbow Pokémon,3.8,199.0,"Pressure, Regenerator",2,Yes
Celebi,251,Psychic,Grass,Time Travel Pokémon,0.6,5.0,Natural-cure,2,Yes
Treecko,252,Grass,,Wood Gecko Pokémon,0.5,5.0,"Overgrow, Unburden",3,No
Grovyle,253,Grass,,Wood Gecko Pokémon,0.9,21.6,"Overgrow, Unburden",3,No
Sceptile,254,Grass,,Forest Pokémon,1.7,52.2,"Overgrow, Unburden",3,No
Torchic,255,Fire,,Chick Pokémon,0.4,2.5,"Blaze, Speed-boost",3,No
Combusken,256,Fire,Fighting,Young Fowl Pokémon,0.9,19.5,"Blaze, Speed-boost",3,No
Blaziken,257,Fire,Fighting,Blaze Pokémon,1.9,52.0,"Blaze, Speed-boost",3,No
Mudkip,258,Water,,Mud Fish Pokémon,0.4,7.6,"Torrent, Damp",3,No
Marshtomp,259,Water,Ground,Mud Fish Pokémon,0.7,28.0,"Torrent, Damp",3,No
Swampert,260,Water,Ground,Mud Fish Pokémon,1.5,81.9,"Torrent, Damp",3,No
Poochyena,261,Dark,,Bite Pokémon,0.5,13.6,"Run-away, Quick-feet, Rattled",3,No
Mightyena,262,Dark,,Bite Pokémon,1.0,37.0,"Intimidate, Quick-feet, Moxie",3,No
Zigzagoon,263,Normal,,Tiny Raccoon Pokémon,0.4,17.5,"Pickup, Gluttony, Quick-feet",3,No
Linoone,264,Normal,,Rushing Pokémon,0.5,32.5,"Pickup, Gluttony, Quick-feet",3,No
Wurmple,265,Bug,,Worm Pokémon,0.3,3.6,"Shield-dust, Run-away",3,No
Silcoon,266,Bug,,Cocoon Pokémon,0.6,10.0,Shed-skin,3,No
Beautifly,267,Bug,Flying,Butterfly Pokémon,1.0,28.4,"Swarm, Rivalry",3,No
Cascoon,268,Bug,,Cocoon Pokémon,0.7,11.5,Shed-skin,3,No
Dustox,269,Bug,Poison,Poison Moth Pokémon,1.2,31.6,"Shield-dust, Compound-eyes",3,No
Lotad,270,Water,Grass,Water Weed Pokémon,0.5,2.6,"Swift-swim, Rain-dish, Own-tempo",3,No
Lombre,271,Water,Grass,Jolly Pokémon,1.2,32.5,"Swift-swim, Rain-dish, Own-tempo",3,No
Ludicolo,272,Water,Grass,Carefree Pokémon,1.5,55.0,"Swift-swim, Rain-dish, Own-tempo",3,No
Seedot,273,Grass,,Acorn Pokémon,0.5,4.0,"Chlorophyll, Early-bird, Pickpocket",3,No
Nuzleaf,274,Grass,Dark,Wily Pokémon,1.0,28.0,"Chlorophyll, Early-bird, Pickpocket",3,No
Shiftry,275,Grass,Dark,Wicked Pokémon,1.3,59.6,"Chlorophyll, Wind-rider, Pickpocket",3,No
Taillow,276,Normal,Flying,Tiny Swallow Pokémon,0.3,2.3,"Guts, Scrappy",3,No
Swellow,277,Normal,Flying,Swallow Pokémon,0.7,19.8,"Guts, Scrappy",3,No
Wingull,278,Water,Flying,Seagull Pokémon,0.6,9.5,"Keen-eye, Hydration, Rain-dish",3,No
Pelipper,279,Water,Flying,Water Bird Pokémon,1.2,28.0,"Keen-eye, Drizzle, Rain-dish",3,No
Ralts,280,Psychic,Fairy,Feeling Pokémon,0.4,6.6,"Synchronize, Trace, Telepathy",3,No
Kirlia,281,Psychic,Fairy,Emotion Pokémon,0.8,20.2,"Synchronize, Trace, Telepathy",3,No
Gardevoir,282,Psychic,Fairy,Embrace Pokémon,1.6,48.4,"Synchronize, Trace, Telepathy",3,No
Surskit,283,Bug,Water,Pond Skater Pokémon,0.5,1.7,"Swift-swim, Rain-dish",3,No
Masquerain,284,Bug,Flying,Eyeball Pokémon,0.8,3.6,"Intimidate, Unnerve",3,No
Shroomish,285,Grass,,Mushroom Pokémon,0.4,4.5,"Effect-spore, Poison-heal, Quick-feet",3,No
Breloom,286,Grass,Fighting,Mushroom Pokémon,1.2,39.2,"Effect-spore, Poison-heal, Technician",3,No
Slakoth,287,Normal,,Slacker Pokémon,0.8,24.0,Truant,3,No
Vigoroth,288,Normal,,Wild Monkey Pokémon,1.4,46.5,Vital-spirit,3,No
Slaking,289,Normal,,Lazy Pokémon,2.0,130.5,Truant,3,No
Nincada,290,Bug,Ground,Trainee Pokémon,0.5,5.5,"Compound-eyes, Run-away",3,No
Ninjask,291,Bug,Flying,Ninja Pokémon,0.8,12.0,"Speed-boost, Infiltrator",3,No
Shedinja,292,Bug,Ghost,Shed Pokémon,0.8,1.2,Wonder-guard,3,No
Whismur,293,Normal,,Whisper Pokémon,0.6,16.3,"Soundproof, Rattled",3,No
Loudred,294,Normal,,Big Voice Pokémon,1.0,40.5,"Soundproof, Scrappy",3,No
Exploud,295,Normal,,Loud Noise Pokémon,1.5,84.0,"Soundproof, Scrappy",3,No
Makuhita,296,Fighting,,Guts Pokémon,1.0,86.4,"Thick-fat, Guts, Sheer-force",3,No
Hariyama,297,Fighting,,Arm Thrust Pokémon,2.3,253.8,"Thick-fat, Guts, Sheer-force",3,No
Azurill,298,Normal,Fairy,Polka Dot Pokémon,0.2,2.0,"Thick-fat, Huge-power, Sap-sipper",3,No
Nosepass,299,Rock,,Compass Pokémon,1.0,97.0,"Sturdy, Magnet-pull, Sand-force",3,No
Skitty,300,Normal,,Kitten Pokémon,0.6,11.0,"Cute-charm, Normalize, Wonder-skin",3,No
Delcatty,301,Normal,,Prim Pokémon,1.1,32.6,"Cute-charm, Normalize, Wonder-skin",3,No
Sableye,302,Dark,Ghost,Darkness Pokémon,0.5,11.0,"Keen-eye, Stall, Prankster",3,No
Mawile,303,Steel,Fairy,Deceiver Pokémon,0.6,11.5,"Hyper-cutter, Intimidate, Sheer-force",3,No
Aron,304,Steel,Rock,Iron Armor Pokémon,0.4,60.0,"Sturdy, Rock-head, Heavy-metal",3,No
Lairon,305,Steel,Rock,Iron Armor Pokémon,0.9,120.0,"Sturdy, Rock-head, Heavy-metal",3,No
Aggron,306,Steel,Rock,Iron Armor Pokémon,2.1,360.0,"Sturdy, Rock-head, Heavy-metal",3,No
Meditite,307,Fighting,Psychic,Meditate Pokémon,0.6,11.2,"Pure-power, Telepathy",3,No
Medicham,308,Fighting,Psychic,Meditate Pokémon,1.3,31.5,"Pure-power, Telepathy",3,No
Electrike,309,Electric,,Lightning Pokémon,0.6,15.2,"Static, Lightning-rod, Minus",3,No
Manectric,310,Electric,,Discharge Pokémon,1.5,40.2,"Static, Lightning-rod, Minus",3,No
Plusle,311,Electric,,Cheering Pokémon,0.4,4.2,"Plus, Lightning-rod",3,No
Minun,312,Electric,,Cheering Pokémon,0.4,4.2,"Minus, Volt-absorb",3,No
Volbeat,313,Bug,,Firefly Pokémon,0.7,17.7,"Illuminate, Swarm, Prankster",3,No
Illumise,314,Bug,,Firefly Pokémon,0.6,17.7,"Oblivious, Tinted-lens, Prankster",3,No
Roselia,315,Grass,Poison,Thorn Pokémon,0.3,2.0,"Natural-cure, Poison-point, Leaf-guard",3,No
Gulpin,316,Poison,,Stomach Pokémon,0.4,10.3,"Liquid-ooze, Sticky-hold, Gluttony",3,No
Swalot,317,Poison,,Poison Bag Pokémon,1.7,80.0,"Liquid-ooze, Sticky-hold, Gluttony",3,No
Carvanha,318,Water,Dark,Savage Pokémon,0.8,20.8,"Rough-skin, Speed-boost",3,No
Sharpedo,319,Water,Dark,Brutal Pokémon,1.8,88.8,"Rough-skin, Speed-boost",3,No
Wailmer,320,Water,,Ball Whale Pokémon,2.0,130.0,"Water-veil, Oblivious, Pressure",3,No
Wailord,321,Water,,Float Whale Pokémon,14.5,398.0,"Water-veil, Oblivious, Pressure",3,No
Numel,322,Fire,Ground,Numb Pokémon,0.7,24.0,"Oblivious, Simple, Own-tempo",3,No
Camerupt,323,Fire,Ground,Eruption Pokémon,1.9,220.0,"Magma-armor, Solid-rock, Anger-point",3,No
Torkoal,324,Fire,,Coal Pokémon,0.5,80.4,"White-smoke, Drought, Shell-armor",3,No
Spoink,325,Psychic,,Bounce Pokémon,0.7,30.6,"Thick-fat, Own-tempo, Gluttony",3,No
Grumpig,326,Psychic,,Manipulate Pokémon,0.9,71.5,"Thick-fat, Own-tempo, Gluttony",3,No
Spinda,327,Normal,,Spot Panda Pokémon,1.1,5.0,"Own-tempo, Tangled-feet, Contrary",3,No
Trapinch,328,Ground,,Ant Pit Pokémon,0.7,15.0,"Hyper-cutter, Arena-trap, Sheer-force",3,No
Vibrava,329,Ground,Dragon,Vibration Pokémon,1.1,15.3,Levitate,3,No
Flygon,330,Ground,Dragon,Mystic Pokémon,2.0,82.0,Levitate,3,No
Cacnea,331,Grass,,Cactus Pokémon,0.4,51.3,"Sand-veil, Water-absorb",3,No
Cacturne,332,Grass,Dark,Scarecrow Pokémon,1.3,77.4,"Sand-veil, Water-absorb",3,No
Swablu,333,Normal,Flying,Cotton Bird Pokémon,0.4,1.2,"Natural-cure, Cloud-nine",3,No
Altaria,334,Dragon,Flying,Humming Pokémon,1.1,20.6,"Natural-cure, Cloud-nine",3,No
Zangoose,335,Normal,,Cat Ferret Pokémon,1.3,40.3,"Immunity, Toxic-boost",3,No
Seviper,336,Poison,,Fang Snake Pokémon,2.7,52.5,"Shed-skin, Infiltrator",3,No
Lunatone,337,Rock,Psychic,Meteorite Pokémon,1.0,168.0,Levitate,3,No
Solrock,338,Rock,Psychic,Meteorite Pokémon,1.2,154.0,Levitate,3,No
Barboach,339,Water,Ground,Whiskers Pokémon,0.4,1.9,"Oblivious, Anticipation, Hydration",3,No
Whiscash,340,Water,Ground,Whiskers Pokémon,0.9,23.6,"Oblivious, Anticipation, Hydration",3,No
Corphish,341,Water,,Ruffian Pokémon,0.6,11.5,"Hyper-cutter, Shell-armor, Adaptability",3,No
Crawdaunt,342,Water,Dark,Rogue Pokémon,1.1,32.8,"Hyper-cutter, Shell-armor, Adaptability",3,No
Baltoy,343,Ground,Psychic,Clay Doll Pokémon,0.5,21.5,Levitate,3,No
Claydol,344,Ground,Psychic,Clay Doll Pokémon,1.5,108.0,Levitate,3,No
Lileep,345,Rock,Grass,Sea Lily Pokémon,1.0,23.8,"Suction-cups, Storm-drain",3,No
Cradily,346,Rock,Grass,Barnacle Pokémon,1.5,60.4,"Suction-cups, Storm-drain",3,No
Anorith,347,Rock,Bug,Old Shrimp Pokémon,0.7,12.5,"Battle-armor, Swift-swim",3,No
Armaldo,348,Rock,Bug,Plate Pokémon,1.5,68.2,"Battle-armor, Swift-swim",3,No
Feebas,349,Water,,Fish Pokémon,0.6,7.4,"Swift-swim, Oblivious, Adaptability",3,No
Milotic,350,Water,,Tender Pokémon,6.2,162.0,"Marvel-scale, Competitive, Cute-charm",3,No
Castform,351,Normal,,Weather Pokémon,0.3,0.8,Forecast,3,No
Kecleon,352,Normal,,Color Swap Pokémon,1.0,22.0,"Color-change, Protean",3,No
Shuppet,353,Ghost,,Puppet Pokémon,0.6,2.3,"Insomnia, Frisk, Cursed-body",3,No
Banette,354,Ghost,,Marionette Pokémon,1.1,12.5,"Insomnia, Frisk, Cursed-body",3,No
Duskull,355,Ghost,,Requiem Pokémon,0.8,15.0,"Levitate, Frisk",3,No
Dusclops,356,Ghost,,Beckon Pokémon,1.6,30.6,"Pressure, Frisk",3,No
Tropius,357,Grass,Flying,Fruit Pokémon,2.0,100.0,"Chlorophyll, Solar-power, Harvest",3,No
Chimecho,358,Psychic,,Wind Chime Pokémon,0.6,1.0,Levitate,3,No
Absol,359,Dark,,Disaster Pokémon,1.2,47.0,"Pressure, Super-luck, Justified",3,No
Wynaut,360,Psychic,,Bright Pokémon,0.6,14.0,"Shadow-tag, Telepathy",3,No
Snorunt,361,Ice,,Snow Hat Pokémon,0.7,16.8,"Inner-focus, Ice-body, Moody",3,No
Glalie,362,Ice,,Face Pokémon,1.5,256.5,"Inner-focus, Ice-body, Moody",3,No
Spheal,363,Ice,Water,Clap Pokémon,0.8,39.5,"Thick-fat, Ice-body, Oblivious",3,No
Sealeo,364,Ice,Water,Ball Roll Pokémon,1.1,87.6,"Thick-fat, Ice-body, Oblivious",3,No
Walrein,365,Ice,Water,Ice Break Pokémon,1.4,150.6,"Thick-fat, Ice-body, Oblivious",3,No
Clamperl,366,Water,,Bivalve Pokémon,0.4,52.5,"Shell-armor, Rattled",3,No
Huntail,367,Water,,Deep Sea Pokémon,1.7,27.0,"Swift-swim, Water-veil",3,No
Gorebyss,368,Water,,South Sea Pokémon,1.8,22.6,"Swift-swim, Hydration",3,No
Relicanth,369,Water,Rock,Longevity Pokémon,1.0,23.4,"Swift-swim, Rock-head, Sturdy",3,No
Luvdisc,370,Water,,Rendezvous Pokémon,0.6,8.7,"Swift-swim, Hydration",3,No
Bagon,371,Dragon,,Rock Head Pokémon,0.6,42.1,"Rock-head, Sheer-force",3,No
Shelgon,372,Dragon,,Endurance Pokémon,1.1,110.5,"Rock-head, Overcoat",3,No
Salamence,373,Dragon,Flying,Dragon Pokémon,1.5,102.6,"Intimidate, Moxie",3,No
Beldum,374,Steel,Psychic,Iron Ball Pokémon,0.6,95.2,"Clear-body, Light-metal",3,No
Metang,375,Steel,Psychic,Iron Claw Pokémon,1.2,202.5,"Clear-body, Light-metal",3,No
Metagross,376,Steel,Psychic,Iron Leg Pokémon,1.6,550.0,"Clear-body, Light-metal",3,No
Regirock,377,Rock,,Rock Peak Pokémon,1.7,230.0,"Clear-body, Sturdy",3,Yes
Regice,378,Ice,,Iceberg Pokémon,1.8,175.0,"Clear-body, Ice-body",3,Yes
Registeel,379,Steel,,Iron Pokémon,1.9,205.0,"Clear-body, Light-metal",3,Yes
Latias,380,Dragon,Psychic,Eon Pokémon,1.4,40.0,Levitate,3,Yes
Latios,381,Dragon,Psychic,Eon Pokémon,2.0,60.0,Levitate,3,Yes
Kyogre,382,Water,,Sea Basin Pokémon,4.5,352.0,Drizzle,3,Yes
Groudon,383,Ground,,Continent Pokémon,3.5,950.0,Drought,3,Yes
Rayquaza,384,Dragon,Flying,Sky High Pokémon,7.0,206.5,Air-lock,3,Yes
Jirachi,385,Steel,Psychic,Wish Pokémon,0.3,1.1,Serene-grace,3,Yes
Deoxys-normal,386,Psychic,,DNA Pokémon,1.7,60.8,Pressure,3,Yes
Turtwig,387,Grass,,Tiny Leaf Pokémon,0.4,10.2,"Overgrow, Shell-armor",4,No
Grotle,388,Grass,,Grove Pokémon,1.1,97.0,"Overgrow, Shell-armor",4,No
Torterra,389,Grass,Ground,Continent Pokémon,2.2,310.0,"Overgrow, Shell-armor",4,No
Chimchar,390,Fire,,Chimp Pokémon,0.5,6.2,"Blaze, Iron-fist",4,No
Monferno,391,Fire,Fighting,Playful Pokémon,0.9,22.0,"Blaze, Iron-fist",4,No
Infernape,392,Fire,Fighting,Flame Pokémon,1.2,55.0,"Blaze, Iron-fist",4,No
Piplup,393,Water,,Penguin Pokémon,0.4,5.2,"Torrent, Competitive",4,No
Prinplup,394,Water,,Penguin Pokémon,0.8,23.0,"Torrent, Competitive",4,No
Empoleon,395,Water,Steel,Emperor Pokémon,1.7,84.5,"Torrent, Competitive",4,No
Starly,396,Normal,Flying,Starling Pokémon,0.3,2.0,"Keen-eye, Reckless",4,No
Staravia,397,Normal,Flying,Starling Pokémon,0.6,15.5,"Intimidate, Reckless",4,No
Staraptor,398,Normal,Flying,Predator Pokémon,1.2,24.9,"Intimidate, Reckless",4,No
Bidoof,399,Normal,,Plump Mouse Pokémon,0.5,20.0,"Simple, Unaware, Moody",4,No
Bibarel,400,Normal,Water,Beaver Pokémon,1.0,31.5,"Simple, Unaware, Moody",4,No
Kricketot,401,Bug,,Cricket Pokémon,0.3,2.2,"Shed-skin, Run-away",4,No
Kricketune,402,Bug,,Cricket Pokémon,1.0,25.5,"Swarm, Technician",4,No
Shinx,403,Electric,,Flash Pokémon,0.5,9.5,"Rivalry, Intimidate, Guts",4,No
Luxio,404,Electric,,Spark Pokémon,0.9,30.5,"Rivalry, Intimidate, Guts",4,No
Luxray,405,Electric,,Gleam Eyes Pokémon,1.4,42.0,"Rivalry, Intimidate, Guts",4,No
Budew,406,Grass,Poison,Bud Pokémon,0.2,1.2,"Natural-cure, Poison-point, Leaf-guard",4,No
Roserade,407,Grass,Poison,Bouquet Pokémon,0.9,14.5,"Natural-cure, Poison-point, Technician",4,No
Cranidos,408,Rock,,Head Butt Pokémon,0.9,31.5,"Mold-breaker, Sheer-force",4,No
Rampardos,409,Rock,,Head Butt Pokémon,1.6,102.5,"Mold-breaker, Sheer-force",4,No
Shieldon,410,Rock,Steel,Shield Pokémon,0.5,57.0,"Sturdy, Soundproof",4,No
Bastiodon,411,Rock,Steel,Shield Pokémon,1.3,149.5,"Sturdy, Soundproof",4,No
Burmy,412,Bug,,Bagworm Pokémon,0.2,3.4,"Shed-skin, Overcoat",4,No
Wormadam-plant,413,Bug,Grass,Bagworm Pokémon,0.5,6.5,"Anticipation, Overcoat",4,No
Mothim,414,Bug,Flying,Moth Pokémon,0.9,23.3,"Swarm, Tinted-lens",4,No
Combee,415,Bug,Flying,Tiny Bee Pokémon,0.3,5.5,"Honey-gather, Hustle",4,No
Vespiquen,416,Bug,Flying,Beehive Pokémon,1.2,38.5,"Pressure, Unnerve",4,No
Pachirisu,417,Electric,,EleSquirrel Pokémon,0.4,3.9,"Run-away, Pickup, Volt-absorb",4,No
Buizel,418,Water,,Sea Weasel Pokémon,0.7,29.5,"Swift-swim, Water-veil",4,No
Floatzel,419,Water,,Sea Weasel Pokémon,1.1,33.5,"Swift-swim, Water-veil",4,No
Cherubi,420,Grass,,Cherry Pokémon,0.4,3.3,Chlorophyll,4,No
Cherrim,421,Grass,,Blossom Pokémon,0.5,9.3,Flower-gift,4,No
Shellos,422,Water,,Sea Slug Pokémon,0.3,6.3,"Sticky-hold, Storm-drain, Sand-force",4,No
Gastrodon,423,Water,Ground,Sea Slug Pokémon,0.9,29.9,"Sticky-hold, Storm-drain, Sand-force",4,No
Ambipom,424,Normal,,Long Tail Pokémon,1.2,20.3,"Technician, Pickup, Skill-link",4,No
Drifloon,425,Ghost,Flying,Balloon Pokémon,0.4,1.2,"Aftermath, Unburden, Flare-boost",4,No
Drifblim,426,Ghost,Flying,Blimp Pokémon,1.2,15.0,"Aftermath, Unburden, Flare-boost",4,No
Buneary,427,Normal,,Rabbit Pokémon,0.4,5.5,"Run-away, Klutz, Limber",4,No
Lopunny,428,Normal,,Rabbit Pokémon,1.2,33.3,"Cute-charm, Klutz, Limber",4,No
Mismagius,429,Ghost,,Magical Pokémon,0.9,4.4,Levitate,4,No
Honchkrow,430,Dark,Flying,Big Boss Pokémon,0.9,27.3,"Insomnia, Super-luck, Moxie",4,No
Glameow,431,Normal,,Catty Pokémon,0.5,3.9,"Limber, Own-tempo, Keen-eye",4,No
Purugly,432,Normal,,Tiger Cat Pokémon,1.0,43.8,"Thick-fat, Own-tempo, Defiant",4,No
Chingling,433,Psychic,,Bell Pokémon,0.2,0.6,Levitate,4,No
Stunky,434,Poison,Dark,Skunk Pokémon,0.4,19.2,"Stench, Aftermath, Keen-eye",4,No
Skuntank,435,Poison,Dark,Skunk Pokémon,1.0,38.0,"Stench, Aftermath, Keen-eye",4,No
Bronzor,436,Steel,Psychic,Bronze Pokémon,0.5,60.5,"Levitate, Heatproof, Heavy-metal",4,No
Bronzong,437,Steel,Psychic,Bronze Bell Pokémon,1.3,187.0,"Levitate, Heatproof, Heavy-metal",4,No
Bonsly,438,Rock,,Bonsai Pokémon,0.5,15.0,"Sturdy, Rock-head, Rattled",4,No
Mime-jr,439,Psychic,Fairy,Mime Pokémon,0.6,13.0,"Soundproof, Filter, Technician",4,No
Happiny,440,Normal,,Playhouse Pokémon,0.6,24.4,"Natural-cure, Serene-grace, Friend-guard",4,No
Chatot,441,Normal,Flying,Music Note Pokémon,0.5,1.9,"Keen-eye, Tangled-feet, Big-pecks",4,No
Spiritomb,442,Ghost,Dark,Forbidden Pokémon,1.0,108.0,"Pressure, Infiltrator",4,No
Gible,443,Dragon,Ground,Land Shark Pokémon,0.7,20.5,"Sand-veil, Rough-skin",4,No
Gabite,444,Dragon,Ground,Cave Pokémon,1.4,56.0,"Sand-veil, Rough-skin",4,No
Garchomp,445,Dragon,Ground,Mach Pokémon,1.9,95.0,"Sand-veil, Rough-skin",4,No
Munchlax,446,Normal,,Big Eater Pokémon,0.6,105.0,"Pickup, Thick-fat, Gluttony",4,No
Riolu,447,Fighting,,Emanation Pokémon,0.7,20.2,"Steadfast, Inner-focus, Prankster",4,No
Lucario,448,Fighting,Steel,Aura Pokémon,1.2,54.0,"Steadfast, Inner-focus, Justified",4,No
Hippopotas,449,Ground,,Hippo Pokémon,0.8,49.5,"Sand-stream, Sand-force",4,No
Hippowdon,450,Ground,,Heavyweight Pokémon,2.0,300.0,"Sand-stream, Sand-force",4,No
Skorupi,451,Poison,Bug,Scorpion Pokémon,0.8,12.0,"Battle-armor, Sniper, Keen-eye",4,No
Drapion,452,Poison,Dark,Ogre Scorpion Pokémon,1.3,61.5,"Battle-armor, Sniper, Keen-eye",4,No
Croagunk,453,Poison,Fighting,Toxic Mouth Pokémon,0.7,23.0,"Anticipation, Dry-skin, Poison-touch",4,No
Toxicroak,454,Poison,Fighting,Toxic Mouth Pokémon,1.3,44.4,"Anticipation, Dry-skin, Poison-touch",4,No
Carnivine,455,Grass,,Bug Catcher Pokémon,1.4,27.0,Levitate,4,No
Finneon,456,Water,,Wing Fish Pokémon,0.4,7.0,"Swift-swim, Storm-drain, Water-veil",4,No
Lumineon,457,Water,,Neon Pokémon,1.2,24.0,"Swift-swim, Storm-drain, Water-veil",4,No
Mantyke,458,Water,Flying,Kite Pokémon,1.0,65.0,"Swift-swim, Water-absorb, Water-veil",4,No
Snover,459,Grass,Ice,Frost Tree Pokémon,1.0,50.5,"Snow-warning, Soundproof",4,No
Abomasnow,460,Grass,Ice,Frost Tree Pokémon,2.2,135.5,"Snow-warning, Soundproof",4,No
Weavile,461,Dark,Ice,Sharp Claw Pokémon,1.1,34.0,"Pressure, Pickpocket",4,No
Magnezone,462,Electric,Steel,Magnet Area Pokémon,1.2,180.0,"Magnet-pull, Sturdy, Analytic",4,No
Lickilicky,463,Normal,,Licking Pokémon,1.7,140.0,"Own-tempo, Oblivious, Cloud-nine",4,No
Rhyperior,464,Ground,Rock,Drill Pokémon,2.4,282.8,"Lightning-rod, Solid-rock, Reckless",4,No
Tangrowth,465,Grass,,Vine Pokémon,2.0,128.6,"Chlorophyll, Leaf-guard, Regenerator",4,No
Electivire,466,Electric,,Thunderbolt Pokémon,1.8,138.6,"Motor-drive, Vital-spirit",4,No
Magmortar,467,Fire,,Blast Pokémon,1.6,68.0,"Flame-body, Vital-spirit",4,No
Togekiss,468,Fairy,Flying,Jubilee Pokémon,1.5,38.0,"Hustle, Serene-grace, Super-luck",4,No
Yanmega,469,Bug,Flying,Ogre Darner Pokémon,1.9,51.5,"Speed-boost, Tinted-lens, Frisk",4,No
Leafeon,470,Grass,,Verdant Pokémon,1.0,25.5,"Leaf-guard, Chlorophyll",4,No
Glaceon,471,Ice,,Fresh Snow Pokémon,0.8,25.9,"Snow-cloak, Ice-body",4,No
Gliscor,472,Ground,Flying,Fang Scorpion Pokémon,2.0,42.5,"Hyper-cutter, Sand-veil, Poison-heal",4,No
Mamoswine,473,Ice,Ground,Twin Tusk Pokémon,2.5,291.0,"Oblivious, Snow-cloak, Thick-fat",4,No
Porygon-z,474,Normal,,Virtual Pokémon,0.9,34.0,"Adaptability, Download, Analytic",4,No
Gallade,475,Psychic,Fighting,Blade Pokémon,1.6,52.0,"Steadfast, Sharpness, Justified",4,No
Probopass,476,Rock,Steel,Compass Pokémon,1.4,340.0,"Sturdy, Magnet-pull, Sand-force",4,No
Dusknoir,477,Ghost,,Gripper Pokémon,2.2,106.6,"Pressure, Frisk",4,No
Froslass,478,Ice,Ghost,Snow Land Pokémon,1.3,26.6,"Snow-cloak, Cursed-body",4,No
Rotom,479,Electric,Ghost,Plasma Pokémon,0.3,0.3,Levitate,4,No
Uxie,480,Psychic,,Knowledge Pokémon,0.3,0.3,Levitate,4,Yes
Mesprit,481,Psychic,,Emotion Pokémon,0.3,0.3,Levitate,4,Yes
Azelf,482,Psychic,,Willpower Pokémon,0.3,0.3,Levitate,4,Yes
Dialga,483,Steel,Dragon,Temporal Pokémon,5.4,683.0,"Pressure, Telepathy",4,Yes
Palkia,484,Water,Dragon,Spatial Pokémon,4.2,336.0,"Pressure, Telepathy",4,Yes
Heatran,485,Fire,Steel,Lava Dome Pokémon,1.7,430.0,"Flash-fire, Flame-body",4,Yes
Regigigas,486,Normal,,Colossal Pokémon,3.7,420.0,Slow-start,4,Yes
Giratina-altered,487,Ghost,Dragon,Renegade Pokémon,4.5,750.0,"Pressure, Telepathy",4,Yes
Cresselia,488,Psychic,,Lunar Pokémon,1.5,85.6,Levitate,4,Yes
Phione,489,Water,,Sea Drifter Pokémon,0.4,3.1,Hydration,4,Yes
Manaphy,490,Water,,Seafaring Pokémon,0.3,1.4,Hydration,4,Yes
Darkrai,491,Dark,,Pitch-Black Pokémon,1.5,50.5,Bad-dreams,4,Yes
Shaymin-land,492,Grass,,Gratitude Pokémon,0.2,2.1,Natural-cure,4,Yes
Arceus,493,Normal,,Alpha Pokémon,3.2,320.0,Multitype,4,Yes
Victini,494,Psychic,Fire,Victory Pokémon,0.4,4.0,Victory-star,5,Yes
Snivy,495,Grass,,Grass Snake Pokémon,0.6,8.1,"Overgrow, Contrary",5,No
Servine,496,Grass,,Grass Snake Pokémon,0.8,16.0,"Overgrow, Contrary",5,No
Serperior,497,Grass,,Regal Pokémon,3.3,63.0,"Overgrow, Contrary",5,No
Tepig,498,Fire,,Fire Pig Pokémon,0.5,9.9,"Blaze, Thick-fat",5,No
Pignite,499,Fire,Fighting,Fire Pig Pokémon,1.0,55.5,"Blaze, Thick-fat",5,No
Emboar,500,Fire,Fighting,Mega Fire Pig Pokémon,1.6,150.0,"Blaze, Reckless",5,No
Oshawott,501,Water,,Sea Otter Pokémon,0.5,5.9,"Torrent, Shell-armor",5,No
Dewott,502,Water,,Discipline Pokémon,0.8,24.5,"Torrent, Shell-armor",5,No
Samurott,503,Water,,Formidable Pokémon,1.5,94.6,"Torrent, Shell-armor",5,No
Patrat,504,Normal,,Scout Pokémon,0.5,11.6,"Run-away, Keen-eye, Analytic",5,No
Watchog,505,Normal,,Lookout Pokémon,1.1,27.0,"Illuminate, Keen-eye, Analytic",5,No
Lillipup,506,Normal,,Puppy Pokémon,0.4,4.1,"Vital-spirit, Pickup, Run-away",5,No
Herdier,507,Normal,,Loyal Dog Pokémon,0.9,14.7,"Intimidate, Sand-rush, Scrappy",5,No
Stoutland,508,Normal,,Big-Hearted Pokémon,1.2,61.0,"Intimidate, Sand-rush, Scrappy",5,No
Purrloin,509,Dark,,Devious Pokémon,0.4,10.1,"Limber, Unburden, Prankster",5,No
Liepard,510,Dark,,Cruel Pokémon,1.1,37.5,"Limber, Unburden, Prankster",5,No
Pansage,511,Grass,,Grass Monkey Pokémon,0.6,10.5,"Gluttony, Overgrow",5,No
Simisage,512,Grass,,Thorn Monkey Pokémon,1.1,30.5,"Gluttony, Overgrow",5,No
Pansear,513,Fire,,High Temp Pokémon,0.6,11.0,"Gluttony, Blaze",5,No
Simisear,514,Fire,,Ember Pokémon,1.0,28.0,"Gluttony, Blaze",5,No
Panpour,515,Water,,Spray Pokémon,0.6,13.5,"Gluttony, Torrent",5,No
Simipour,516,Water,,Geyser Pokémon,1.0,29.0,"Gluttony, Torrent",5,No
Munna,517,Psychic,,Dream Eater Pokémon,0.6,23.3,"Forewarn, Synchronize, Telepathy",5,No
Musharna,518,Psychic,,Drowsing Pokémon,1.1,60.5,"Forewarn, Synchronize, Telepathy",5,No
Pidove,519,Normal,Flying,Tiny Pigeon Pokémon,0.3,2.1,"Big-pecks, Super-luck, Rivalry",5,No
Tranquill,520,Normal,Flying,Wild Pigeon Pokémon,0.6,15.0,"Big-pecks, Super-luck, Rivalry",5,No
Unfezant,521,Normal,Flying,Proud Pokémon,1.2,29.0,"Big-pecks, Super-luck, Rivalry",5,No
Blitzle,522,Electric,,Electrified Pokémon,0.8,29.8,"Lightning-rod, Motor-drive, Sap-sipper",5,No
Zebstrika,523,Electric,,Thunderbolt Pokémon,1.6,79.5,"Lightning-rod, Motor-drive, Sap-sipper",5,No
Roggenrola,524,Rock,,Mantle Pokémon,0.4,18.0,"Sturdy, Weak-armor, Sand-force",5,No
Boldore,525,Rock,,Ore Pokémon,0.9,102.0,"Sturdy, Weak-armor, Sand-force",5,No
Gigalith,526,Rock,,Compressed Pokémon,1.7,260.0,"Sturdy, Sand-stream, Sand-force",5,No
Woobat,527,Psychic,Flying,Bat Pokémon,0.4,2.1,"Unaware, Klutz, Simple",5,No
Swoobat,528,Psychic,Flying,Courting Pokémon,0.9,10.5,"Unaware, Klutz, Simple",5,No
Drilbur,529,Ground,,Mole Pokémon,0.3,8.5,"Sand-rush, Sand-force, Mold-breaker",5,No
Excadrill,530,Ground,Steel,Subterrene Pokémon,0.7,40.4,"Sand-rush, Sand-force, Mold-breaker",5,No
Audino,531,Normal,,Hearing Pokémon,1.1,31.0,"Healer, Regenerator, Klutz",5,No
Timburr,532,Fighting,,Muscular Pokémon,0.6,12.5,"Guts, Sheer-force, Iron-fist",5,No
Gurdurr,533,Fighting,,Muscular Pokémon,1.2,40.0,"Guts, Sheer-force, Iron-fist",5,No
Conkeldurr,534,Fighting,,Muscular Pokémon,1.4,87.0,"Guts, Sheer-force, Iron-fist",5,No
Tympole,535,Water,,Tadpole Pokémon,0.5,4.5,"Swift-swim, Hydration, Water-absorb",5,No
Palpitoad,536,Water,Ground,Vibration Pokémon,0.8,17.0,"Swift-swim, Hydration, Water-absorb",5,No
Seismitoad,537,Water,Ground,Vibration Pokémon,1.5,62.0,"Swift-swim, Poison-touch, Water-absorb",5,No
Throh,538,Fighting,,Judo Pokémon,1.3,55.5,"Guts, Inner-focus, Mold-breaker",5,No
Sawk,539,Fighting,,Karate Pokémon,1.4,51.0,"Sturdy, Inner-focus, Mold-breaker",5,No
Sewaddle,540,Bug,Grass,Sewing Pokémon,0.3,2.5,"Swarm, Chlorophyll, Overcoat",5,No
Swadloon,541,Bug,Grass,Leaf-Wrapped Pokémon,0.5,7.3,"Leaf-guard, Chlorophyll, Overcoat",5,No
Leavanny,542,Bug,Grass,Nurturing Pokémon,1.2,20.5,"Swarm, Chlorophyll, Overcoat",5,No
Venipede,543,Bug,Poison,Centipede Pokémon,0.4,5.3,"Poison-point, Swarm, Speed-boost",5,No
Whirlipede,544,Bug,Poison,Curlipede Pokémon,1.2,58.5,"Poison-point, Swarm, Speed-boost",5,No
Scolipede,545,Bug,Poison,Megapede Pokémon,2.5,200.5,"Poison-point, Swarm, Speed-boost",5,No
Cottonee,546,Grass,Fairy,Cotton Puff Pokémon,0.3,0.6,"Prankster, Infiltrator, Chlorophyll",5,No
Whimsicott,547,Grass,Fairy,Windveiled Pokémon,0.7,6.6,"Prankster, Infiltrator, Chlorophyll",5,No
Petilil,548,Grass,,Bulb Pokémon,0.5,6.6,"Chlorophyll, Own-tempo, Leaf-guard",5,No
Lilligant,549,Grass,,Flowering Pokémon,1.1,16.3,"Chlorophyll, Own-tempo, Leaf-guard",5,No
Basculin-red-striped,550,Water,,Hostile Pokémon,1.0,18.0,"Reckless, Adaptability, Mold-breaker",5,No
Sandile,551,Ground,Dark,Desert Croc Pokémon,0.7,15.2,"Intimidate, Moxie, Anger-point",5,No
Krokorok,552,Ground,Dark,Desert Croc Pokémon,1.0,33.4,"Intimidate, Moxie, Anger-point",5,No
Krookodile,553,Ground,Dark,Intimidation Pokémon,1.5,96.3,"Intimidate, Moxie, Anger-point",5,No
Darumaka,554,Fire,,Zen Charm Pokémon,0.6,37.5,"Hustle, Inner-focus",5,No
Darmanitan-standard,555,Fire,,Blazing Pokémon,1.3,92.9,"Sheer-force, Zen-mode",5,No
Maractus,556,Grass,,Cactus Pokémon,1.0,28.0,"Water-absorb, Chlorophyll, Storm-drain",5,No
Dwebble,557,Bug,Rock,Rock Inn Pokémon,0.3,14.5,"Sturdy, Shell-armor, Weak-armor",5,No
Crustle,558,Bug,Rock,Stone Home Pokémon,1.4,200.0,"Sturdy, Shell-armor, Weak-armor",5,No
Scraggy,559,Dark,Fighting,Shedding Pokémon,0.6,11.8,"Shed-skin, Moxie, Intimidate",5,No
Scrafty,560,Dark,Fighting,Hoodlum Pokémon,1.1,30.0,"Shed-skin, Moxie, Intimidate",5,No
Sigilyph,561,Psychic,Flying,Avianoid Pokémon,1.4,14.0,"Wonder-skin, Magic-guard, Tinted-lens",5,No
Yamask,562,Ghost,,Spirit Pokémon,0.5,1.5,Mummy,5,No
Cofagrigus,563,Ghost,,Coffin Pokémon,1.7,76.5,Mummy,5,No
Tirtouga,564,Water,Rock,Prototurtle Pokémon,0.7,16.5,"Solid-rock, Sturdy, Swift-swim",5,No
Carracosta,565,Water,Rock,Prototurtle Pokémon,1.2,81.0,"Solid-rock, Sturdy, Swift-swim",5,No
Archen,566,Rock,Flying,First Bird Pokémon,0.5,9.5,Defeatist,5,No
Archeops,567,Rock,Flying,First Bird Pokémon,1.4,32.0,Defeatist,5,No
Trubbish,568,Poison,,Trash Bag Pokémon,0.6,31.0,"Stench, Sticky-hold, Aftermath",5,No
Garbodor,569,Poison,,Trash Heap Pokémon,1.9,107.3,"Stench, Weak-armor, Aftermath",5,No
Zorua,570,Dark,,Tricky Fox Pokémon,0.7,12.5,Illusion,5,No
Zoroark,571,Dark,,Illusion Fox Pokémon,1.6,81.1,Illusion,5,No
Minccino,572,Normal,,Chinchilla Pokémon,0.4,5.8,"Cute-charm, Technician, Skill-link",5,No
Cinccino,573,Normal,,Scarf Pokémon,0.5,7.5,"Cute-charm, Technician, Skill-link",5,No
Gothita,574,Psychic,,Fixation Pokémon,0.4,5.8,"Frisk, Competitive, Shadow-tag",5,No
Gothorita,575,Psychic,,Manipulate Pokémon,0.7,18.0,"Frisk, Competitive, Shadow-tag",5,No
Gothitelle,576,Psychic,,Astral Body Pokémon,1.5,44.0,"Frisk, Competitive, Shadow-tag",5,No
Solosis,577,Psychic,,Cell Pokémon,0.3,1.0,"Overcoat, Magic-guard, Regenerator",5,No
Duosion,578,Psychic,,Mitosis Pokémon,0.6,8.0,"Overcoat, Magic-guard, Regenerator",5,No
Reuniclus,579,Psychic,,Multiplying Pokémon,1.0,20.1,"Overcoat, Magic-guard, Regenerator",5,No
Ducklett,580,Water,Flying,Water Bird Pokémon,0.5,5.5,"Keen-eye, Big-pecks, Hydration",5,No
Swanna,581,Water,Flying,White Bird Pokémon,1.3,24.2,"Keen-eye, Big-pecks, Hydration",5,No
Vanillite,582,Ice,,Fresh Snow Pokémon,0.4,5.7,"Ice-body, Snow-cloak, Weak-armor",5,No
Vanillish,583,Ice,,Icy Snow Pokémon,1.1,41.0,"Ice-body, Snow-cloak, Weak-armor",5,No
Vanilluxe,584,Ice,,Snowstorm Pokémon,1.3,57.5,"Ice-body, Snow-warning, Weak-armor",5,No
Deerling,585,Normal,Grass,Season Pokémon,0.6,19.5,"Chlorophyll, Sap-sipper, Serene-grace",5,No
Sawsbuck,586,Normal,Grass,Season Pokémon,1.9,92.5,"Chlorophyll, Sap-sipper, Serene-grace",5,No
Emolga,587,Electric,Flying,Sky Squirrel Pokémon,0.4,5.0,"Static, Motor-drive",5,No
Karrablast,588,Bug,,Clamping Pokémon,0.5,5.9,"Swarm, Shed-skin, No-guard",5,No
Escavalier,589,Bug,Steel,Cavalry Pokémon,1.0,33.0,"Swarm, Shell-armor, Overcoat",5,No
Foongus,590,Grass,Poison,Mushroom Pokémon,0.2,1.0,"Effect-spore, Regenerator",5,No
Amoonguss,591,Grass,Poison,Mushroom Pokémon,0.6,10.5,"Effect-spore, Regenerator",5,No
Frillish,592,Water,Ghost,Floating Pokémon,1.2,33.0,"Water-absorb, Cursed-body, Damp",5,No
Jellicent,593,Water,Ghost,Floating Pokémon,2.2,135.0,"Water-absorb, Cursed-body, Damp",5,No
Alomomola,594,Water,,Caring Pokémon,1.2,31.6,"Healer, Hydration, Regenerator",5,No
Joltik,595,Bug,Electric,Attaching Pokémon,0.1,0.6,"Compound-eyes, Unnerve, Swarm",5,No
Galvantula,596,Bug,Electric,EleSpider Pokémon,0.8,14.3,"Compound-eyes, Unnerve, Swarm",5,No
Ferroseed,597,Grass,Steel,Thorn Seed Pokémon,0.6,18.8,Iron-barbs,5,No
Ferrothorn,598,Grass,Steel,Thorn Pod Pokémon,1.0,110.0,"Iron-barbs, Anticipation",5,No
Klink,599,Steel,,Gear Pokémon,0.3,21.0,"Plus, Minus, Clear-body",5,No
Klang,600,Steel,,Gear Pokémon,0.6,51.0,"Plus, Minus, Clear-body",5,No
Klinklang,601,Steel,,Gear Pokémon,0.6,81.0,"Plus, Minus, Clear-body",5,No
Tynamo,602,Electric,,EleFish Pokémon,0.2,0.3,Levitate,5,No
Eelektrik,603,Electric,,EleFish Pokémon,1.2,22.0,Levitate,5,No
Eelektross,604,Electric,,EleFish Pokémon,2.1,80.5,Levitate,5,No
Elgyem,605,Psychic,,Cerebral Pokémon,0.5,9.0,"Telepathy, Synchronize, Analytic",5,No
Beheeyem,606,Psychic,,Cerebral Pokémon,1.0,34.5,"Telepathy, Synchronize, Analytic",5,No
Litwick,607,Ghost,Fire,Candle Pokémon,0.3,3.1,"Flash-fire, Flame-body, Infiltrator",5,No
Lampent,608,Ghost,Fire,Lamp Pokémon,0.6,13.0,"Flash-fire, Flame-body, Infiltrator",5,No
Chandelure,609,Ghost,Fire,Luring Pokémon,1.0,34.3,"Flash-fire, Flame-body, Infiltrator",5,No
Axew,610,Dragon,,Tusk Pokémon,0.6,18.0,"Rivalry, Mold-breaker, Unnerve",5,No
Fraxure,611,Dragon,,Axe Jaw Pokémon,1.0,36.0,"Rivalry, Mold-breaker, Unnerve",5,No
Haxorus,612,Dragon,,Axe Jaw Pokémon,1.8,105.5,"Rivalry, Mold-breaker, Unnerve",5,No
Cubchoo,613,Ice,,Chill Pokémon,0.5,8.5,"Snow-cloak, Slush-rush, Rattled",5,No
Beartic,614,Ice,,Freezing Pokémon,2.6,260.0,"Snow-cloak, Slush-rush, Swift-swim",5,No
Cryogonal,615,Ice,,Crystallizing Pokémon,1.1,148.0,Levitate,5,No
Shelmet,616,Bug,,Snail Pokémon,0.4,7.7,"Hydration, Shell-armor, Overcoat",5,No
Accelgor,617,Bug,,Shell Out Pokémon,0.8,25.3,"Hydration, Sticky-hold, Unburden",5,No
Stunfisk,618,Ground,Electric,Trap Pokémon,0.7,11.0,"Static, Limber, Sand-veil",5,No
Mienfoo,619,Fighting,,Martial Arts Pokémon,0.9,20.0,"Inner-focus, Regenerator, Reckless",5,No
Mienshao,620,Fighting,,Martial Arts Pokémon,1.4,35.5,"Inner-focus, Regenerator, Reckless",5,No
Druddigon,621,Dragon,,Cave Pokémon,1.6,139.0,"Rough-skin, Sheer-force, Mold-breaker",5,No
Golett,622,Ground,Ghost,Automaton Pokémon,1.0,92.0,"Iron-fist, Klutz, No-guard",5,No
Golurk,623,Ground,Ghost,Automaton Pokémon,2.8,330.0,"Iron-fist, Klutz, No-guard",5,No
Pawniard,624,Dark,Steel,Sharp Blade Pokémon,0.5,10.2,"Defiant, Inner-focus, Pressure",5,No
Bisharp,625,Dark,Steel,Sword Blade Pokémon,1.6,70.0,"Defiant, Inner-focus, Pressure",5,No
Bouffalant,626,Normal,,Bash Buffalo Pokémon,1.6,94.6,"Reckless, Sap-sipper, Soundproof",5,No
Rufflet,627,Normal,Flying,Eaglet Pokémon,0.5,10.5,"Keen-eye, Sheer-force, Hustle",5,No
Braviary,628,Normal,Flying,Valiant Pokémon,1.5,41.0,"Keen-eye, Sheer-force, Defiant",5,No
Vullaby,629,Dark,Flying,Diapered Pokémon,0.5,9.0,"Big-pecks, Overcoat, Weak-armor",5,No
Mandibuzz,630,Dark,Flying,Bone Vulture Pokémon,1.2,39.5,"Big-pecks, Overcoat, Weak-armor",5,No
Heatmor,631,Fire,,Anteater Pokémon,1.4,58.0,"Gluttony, Flash-fire, White-smoke",5,No
Durant,632,Bug,Steel,Iron Ant Pokémon,0.3,33.0,"Swarm, Hustle, Truant",5,No
Deino,633,Dark,Dragon,Irate Pokémon,0.8,17.3,Hustle,5,No
Zweilous,634,Dark,Dragon,Hostile Pokémon,1.4,50.0,Hustle,5,No
Hydreigon,635,Dark,Dragon,Brutal Pokémon,1.8,160.0,Levitate,5,No
Larvesta,636,Bug,Fire,Torch Pokémon,1.1,28.8,"Flame-body, Swarm",5,No
Volcarona,637,Bug,Fire,Sun Pokémon,1.6,46.0,"Flame-body, Swarm",5,No
Cobalion,638,Steel,Fighting,Iron Will Pokémon,2.1,250.0,Justified,5,Yes
Terrakion,639,Rock,Fighting,Cavern Pokémon,1.9,260.0,Justified,5,Yes
Virizion,640,Grass,Fighting,Grassland Pokémon,2.0,200.0,Justified,5,Yes
Tornadus-incarnate,641,Flying,,Cyclone Pokémon,1.5,63.0,"Prankster, Defiant",5,Yes
Thundurus-incarnate,642,Electric,Flying,Bolt Strike Pokémon,1.5,61.0,"Prankster, Defiant",5,Yes
Reshiram,643,Dragon,Fire,Vast White Pokémon,3.2,330.0,Turboblaze,5,Yes
Zekrom,644,Dragon,Electric,Deep Black Pokémon,2.9,345.0,Teravolt,5,Yes
Landorus-incarnate,645,Ground,Flying,Abundance Pokémon,1.5,68.0,"Sand-force, Sheer-force",5,Yes
Kyurem,646,Dragon,Ice,Boundary Pokémon,3.0,325.0,Pressure,5,Yes
Keldeo-ordinary,647,Water,Fighting,Colt Pokémon,1.4,48.5,Justified,5,Yes
Meloetta-aria,648,Normal,Psychic,Melody Pokémon,0.6,6.5,Serene-grace,5,Yes
Genesect,649,Bug,Steel,Paleozoic Pokémon,1.5,82.5,Download,5,Yes
Chespin,650,Grass,,Spiny Nut Pokémon,0.4,9.0,"Overgrow, Bulletproof",6,No
Quilladin,651,Grass,,Spiny Armor Pokémon,0.7,29.0,"Overgrow, Bulletproof",6,No
Chesnaught,652,Grass,Fighting,Spiny Armor Pokémon,1.6,90.0,"Overgrow, Bulletproof",6,No
Fennekin,653,Fire,,Fox Pokémon,0.4,9.4,"Blaze, Magician",6,No
Braixen,654,Fire,,Fox Pokémon,1.0,14.5,"Blaze, Magician",6,No
Delphox,655,Fire,Psychic,Fox Pokémon,1.5,39.0,"Blaze, Magician",6,No
Froakie,656,Water,,Bubble Frog Pokémon,0.3,7.0,"Torrent, Protean",6,No
Frogadier,657,Water,,Bubble Frog Pokémon,0.6,10.9,"Torrent, Protean",6,No
Greninja,658,Water,Dark,Ninja Pokémon,1.5,40.0,"Torrent, Protean",6,No
Bunnelby,659,Normal,,Digging Pokémon,0.4,5.0,"Pickup, Cheek-pouch, Huge-power",6,No
Diggersby,660,Normal,Ground,Digging Pokémon,1.0,42.4,"Pickup, Cheek-pouch, Huge-power",6,No
Fletchling,661,Normal,Flying,Tiny Robin Pokémon,0.3,1.7,"Big-pecks, Gale-wings",6,No
Fletchinder,662,Fire,Flying,Ember Pokémon,0.7,16.0,"Flame-body, Gale-wings",6,No
Talonflame,663,Fire,Flying,Scorching Pokémon,1.2,24.5,"Flame-body, Gale-wings",6,No
Scatterbug,664,Bug,,Scatterdust Pokémon,0.3,2.5,"Shield-dust, Compound-eyes, Friend-guard",6,No
Spewpa,665,Bug,,Scatterdust Pokémon,0.3,8.4,"Shed-skin, Friend-guard",6,No
Vivillon,666,Bug,Flying,Scale Pokémon,1.2,17.0,"Shield-dust, Compound-eyes, Friend-guard",6,No
Litleo,667,Fire,Normal,Lion Cub Pokémon,0.6,13.5,"Rivalry, Unnerve, Moxie",6,No
Pyroar,668,Fire,Normal,Royal Pokémon,1.5,81.5,"Rivalry, Unnerve, Moxie",6,No
Flabebe,669,Fairy,,Single Bloom Pokémon,0.1,0.1,"Flower-veil, Symbiosis",6,No
Floette,670,Fairy,,Single Bloom Pokémon,0.2,0.9,"Flower-veil, Symbiosis",6,No
Florges,671,Fairy,,Garden Pokémon,1.1,10.0,"Flower-veil, Symbiosis",6,No
Skiddo,672,Grass,,Mount Pokémon,0.9,31.0,"Sap-sipper, Grass-pelt",6,No
Gogoat,673,Grass,,Mount Pokémon,1.7,91.0,"Sap-sipper, Grass-pelt",6,No
Pancham,674,Fighting,,Playful Pokémon,0.6,8.0,"Iron-fist, Mold-breaker, Scrappy",6,No
Pangoro,675,Fighting,Dark,Daunting Pokémon,2.1,136.0,"Iron-fist, Mold-breaker, Scrappy",6,No
Furfrou,676,Normal,,Poodle Pokémon,1.2,28.0,Fur-coat,6,No
Espurr,677,Psychic,,Restraint Pokémon,0.3,3.5,"Keen-eye, Infiltrator, Own-tempo",6,No
Meowstic-male,678,Psychic,,Constraint Pokémon,0.6,8.5,"Keen-eye, Infiltrator, Prankster",6,No
Honedge,679,Steel,Ghost,Sword Pokémon,0.8,2.0,No-guard,6,No
Doublade,680,Steel,Ghost,Sword Pokémon,0.8,4.5,No-guard,6,No
Aegislash-shield,681,Steel,Ghost,Royal Sword Pokémon,1.7,53.0,Stance-change,6,No
Spritzee,682,Fairy,,Perfume Pokémon,0.2,0.5,"Healer, Aroma-veil",6,No
Aromatisse,683,Fairy,,Fragrance Pokémon,0.8,15.5,"Healer, Aroma-veil",6,No
Swirlix,684,Fairy,,Cotton Candy Pokémon,0.4,3.5,"Sweet-veil, Unburden",6,No
Slurpuff,685,Fairy,,Meringue Pokémon,0.8,5.0,"Sweet-veil, Unburden",6,No
Inkay,686,Dark,Psychic,Revolving Pokémon,0.4,3.5,"Contrary, Suction-cups, Infiltrator",6,No
Malamar,687,Dark,Psychic,Overturning Pokémon,1.5,47.0,"Contrary, Suction-cups, Infiltrator",6,No
Binacle,688,Rock,Water,Two-Handed Pokémon,0.5,31.0,"Tough-claws, Sniper, Pickpocket",6,No
Barbaracle,689,Rock,Water,Collective Pokémon,1.3,96.0,"Tough-claws, Sniper, Pickpocket",6,No
Skrelp,690,Poison,Water,Mock Kelp Pokémon,0.5,7.3,"Poison-point, Poison-touch, Adaptability",6,No
Dragalge,691,Poison,Dragon,Mock Kelp Pokémon,1.8,81.5,"Poison-point, Poison-touch, Adaptability",6,No
Clauncher,692,Water,,Water Gun Pokémon,0.5,8.3,Mega-launcher,6,No
Clawitzer,693,Water,,Howitzer Pokémon,1.3,35.3,Mega-launcher,6,No
Helioptile,694,Electric,Normal,Generator Pokémon,0.5,6.0,"Dry-skin, Sand-veil, Solar-power",6,No
Heliolisk,695,Electric,Normal,Generator Pokémon,1.0,21.0,"Dry-skin, Sand-veil, Solar-power",6,No
Tyrunt,696,Rock,Dragon,Royal Heir Pokémon,0.8,26.0,"Strong-jaw, Sturdy",6,No
Tyrantrum,697,Rock,Dragon,Despot Pokémon,2.5,270.0,"Strong-jaw, Rock-head",6,No
Amaura,698,Rock,Ice,Tundra Pokémon,1.3,25.2,"Refrigerate, Snow-warning",6,No
Aurorus,699,Rock,Ice,Tundra Pokémon,2.7,225.0,"Refrigerate, Snow-warning",6,No
Sylveon,700,Fairy,,Intertwining Pokémon,1.0,23.5,"Cute-charm, Pixilate",6,No
Hawlucha,701,Fighting,Flying,Wrestling Pokémon,0.8,21.5,"Limber, Unburden, Mold-breaker",6,No
Dedenne,702,Electric,Fairy,Antenna Pokémon,0.2,2.2,"Cheek-pouch, Pickup, Plus",6,No
Carbink,703,Rock,Fairy,Jewel Pokémon,0.3,5.7,"Clear-body, Sturdy",6,No
Goomy,704,Dragon,,Soft Tissue Pokémon,0.3,2.8,"Sap-sipper, Hydration, Gooey",6,No
Sliggoo,705,Dragon,,Soft Tissue Pokémon,0.8,17.5,"Sap-sipper, Hydration, Gooey",6,No
Goodra,706,Dragon,,Dragon Pokémon,2.0,150.5,"Sap-sipper, Hydration, Gooey",6,No
Klefki,707,Steel,Fairy,Key Ring Pokémon,0.2,3.0,"Prankster, Magician",6,No
Phantump,708,Ghost,Grass,Stump Pokémon,0.4,7.0,"Natural-cure, Frisk, Harvest",6,No
Trevenant,709,Ghost,Grass,Elder Tree Pokémon,1.5,71.0,"Natural-cure, Frisk, Harvest",6,No
Pumpkaboo-average,710,Ghost,Grass,Pumpkin Pokémon,0.4,5.0,"Pickup, Frisk, Insomnia",6,No
Gourgeist-average,711,Ghost,Grass,Pumpkin Pokémon,0.9,12.5,"Pickup, Frisk, Insomnia",6,No
Bergmite,712,Ice,,Ice Chunk Pokémon,1.0,99.5,"Own-tempo, Ice-body, Sturdy",6,No
Avalugg,713,Ice,,Iceberg Pokémon,2.0,505.0,"Own-tempo, Ice-body, Sturdy",6,No
Noibat,714,Flying,Dragon,Sound Wave Pokémon,0.5,8.0,"Frisk, Infiltrator, Telepathy",6,No
Noivern,715,Flying,Dragon,Sound Wave Pokémon,1.5,85.0,"Frisk, Infiltrator, Telepathy",6,No
Xerneas,716,Fairy,,Life Pokémon,3.0,215.0,Fairy-aura,6,Yes
Yveltal,717,Dark,Flying,Destruction Pokémon,5.8,203.0,Dark-aura,6,Yes
Zygarde-50,718,Dragon,Ground,Order Pokémon,5.0,305.0,Aura-break,6,Yes
Diancie,719,Rock,Fairy,Jewel Pokémon,0.7,8.8,Clear-body,6,Yes
Hoopa,720,Psychic,Ghost,Mischief Pokémon,0.5,9.0,Magician,6,Yes
Volcanion,721,Fire,Water,Steam Pokémon,1.7,195.0,Water-absorb,6,Yes
Rowlet,722,Grass,Flying,Grass Quill Pokémon,0.3,1.5,"Overgrow, Long-reach",7,No
Dartrix,723,Grass,Flying,Blade Quill Pokémon,0.7,16.0,"Overgrow, Long-reach",7,No
Decidueye,724,Grass,Ghost,Arrow Quill Pokémon,1.6,36.6,"Overgrow, Long-reach",7,No
Litten,725,Fire,,Fire Cat Pokémon,0.4,4.3,"Blaze, Intimidate",7,No
Torracat,726,Fire,,Fire Cat Pokémon,0.7,25.0,"Blaze, Intimidate",7,No
Incineroar,727,Fire,Dark,Heel Pokémon,1.8,83.0,"Blaze, Intimidate",7,No
Popplio,728,Water,,Sea Lion Pokémon,0.4,7.5,"Torrent, Liquid-voice",7,No
Brionne,729,Water,,Pop Star Pokémon,0.6,17.5,"Torrent, Liquid-voice",7,No
Primarina,730,Water,Fairy,Soloist Pokémon,1.8,44.0,"Torrent, Liquid-voice",7,No
Pikipek,731,Normal,Flying,Woodpecker Pokémon,0.3,1.2,"Keen-eye, Skill-link, Pickup",7,No
Trumbeak,732,Normal,Flying,Bugle Beak Pokémon,0.6,14.8,"Keen-eye, Skill-link, Pickup",7,No
Toucannon,733,Normal,Flying,Cannon Pokémon,1.1,26.0,"Keen-eye, Skill-link, Sheer-force",7,No
Yungoos,734,Normal,,Loitering Pokémon,0.4,6.0,"Stakeout, Strong-jaw, Adaptability",7,No
Gumshoos,735,Normal,,Stakeout Pokémon,0.7,14.2,"Stakeout, Strong-jaw, Adaptability",7,No
Grubbin,736,Bug,,Larva Pokémon,0.4,4.4,Swarm,7,No
Charjabug,737,Bug,Electric,Battery Pokémon,0.5,10.5,Battery,7,No
Vikavolt,738,Bug,Electric,Stag Beetle Pokémon,1.5,45.0,Levitate,7,No
Crabrawler,739,Fighting,,Boxing Pokémon,0.6,7.0,"Hyper-cutter, Iron-fist, Anger-point",7,No
Crabominable,740,Fighting,Ice,Woolly Crab Pokémon,1.7,180.0,"Hyper-cutter, Iron-fist, Anger-point",7,No
Oricorio-baile,741,Fire,Flying,Dancing Pokémon,0.6,3.4,Dancer,7,No
Cutiefly,742,Bug,Fairy,Bee Fly Pokémon,0.1,0.2,"Honey-gather, Shield-dust, Sweet-veil",7,No
Ribombee,743,Bug,Fairy,Bee Fly Pokémon,0.2,0.5,"Honey-gather, Shield-dust, Sweet-veil",7,No
Rockruff,744,Rock,,Puppy Pokémon,0.5,9.2,"Keen-eye, Vital-spirit, Steadfast",7,No
Lycanroc-midday,745,Rock,,Wolf Pokémon,0.8,25.0,"Keen-eye, Sand-rush, Steadfast",7,No
Wishiwashi-solo,746,Water,,Small Fry Pokémon,0.2,0.3,Schooling,7,No
Mareanie,747,Poison,Water,Brutal Star Pokémon,0.4,8.0,"Merciless, Limber, Regenerator",7,No
Toxapex,748,Poison,Water,Brutal Star Pokémon,0.7,14.5,"Merciless, Limber, Regenerator",7,No
Mudbray,749,Ground,,Donkey Pokémon,1.0,110.0,"Own-tempo, Stamina, Inner-focus",7,No
Mudsdale,750,Ground,,Draft Horse Pokémon,2.5,920.0,"Own-tempo, Stamina, Inner-focus",7,No
Dewpider,751,Water,Bug,Water Bubble Pokémon,0.3,4.0,"Water-bubble, Water-absorb",7,No
Araquanid,752,Water,Bug,Water Bubble Pokémon,1.8,82.0,"Water-bubble, Water-absorb",7,No
Fomantis,753,Grass,,Sickle Grass Pokémon,0.3,1.5,"Leaf-guard, Contrary",7,No
Lurantis,754,Grass,,Bloom Sickle Pokémon,0.9,18.5,"Leaf-guard, Contrary",7,No
Morelull,755,Grass,Fairy,Illuminating Pokémon,0.2,1.5,"Illuminate, Effect-spore, Rain-dish",7,No
Shiinotic,756,Grass,Fairy,Illuminating Pokémon,1.0,11.5,"Illuminate, Effect-spore, Rain-dish",7,No
Salandit,757,Poison,Fire,Toxic Lizard Pokémon,0.6,4.8,"Corrosion, Oblivious",7,No
Salazzle,758,Poison,Fire,Toxic Lizard Pokémon,1.2,22.2,"Corrosion, Oblivious",7,No
Stufful,759,Normal,Fighting,Flailing Pokémon,0.5,6.8,"Fluffy, Klutz, Cute-charm",7,No
Bewear,760,Normal,Fighting,Strong Arm Pokémon,2.1,135.0,"Fluffy, Klutz, Unnerve",7,No
Bounsweet,761,Grass,,Fruit Pokémon,0.3,3.2,"Leaf-guard, Oblivious, Sweet-veil",7,No
Steenee,762,Grass,,Fruit Pokémon,0.7,8.2,"Leaf-guard, Oblivious, Sweet-veil",7,No
Tsareena,763,Grass,,Fruit Pokémon,1.2,21.4,"Leaf-guard, Queenly-majesty, Sweet-veil",7,No
Comfey,764,Fairy,,Posy Picker Pokémon,0.1,0.3,"Flower-veil, Triage, Natural-cure",7,No
Oranguru,765,Normal,Psychic,Sage Pokémon,1.5,76.0,"Inner-focus, Telepathy, Symbiosis",7,No
Passimian,766,Fighting,,Teamwork Pokémon,2.0,82.8,"Receiver, Defiant",7,No
Wimpod,767,Bug,Water,Turn Tail Pokémon,0.5,12.0,Wimp-out,7,No
Golisopod,768,Bug,Water,Hard Scale Pokémon,2.0,108.0,Emergency-exit,7,No
Sandygast,769,Ghost,Ground,Sand Heap Pokémon,0.5,70.0,"Water-compaction, Sand-veil",7,No
Palossand,770,Ghost,Ground,Sand Castle Pokémon,1.3,250.0,"Water-compaction, Sand-veil",7,No
Pyukumuku,771,Water,,Sea Cucumber Pokémon,0.3,1.2,"Innards-out, Unaware",7,No
Type-null,772,Normal,,Synthetic Pokémon,1.9,120.5,Battle-armor,7,No
Silvally,773,Normal,,Synthetic Pokémon,2.3,100.5,Rks-system,7,Yes
Minior-red-meteor,774,Rock,Flying,Meteor Pokémon,0.3,40.0,Shields-down,7,No
Komala,775,Normal,,Drowsing Pokémon,0.4,19.9,Comatose,7,No
Turtonator,776,Fire,Dragon,Blast Turtle Pokémon,2.0,212.0,Shell-armor,7,No
Togedemaru,777,Electric,Steel,Roly-Poly Pokémon,0.3,3.3,"Iron-barbs, Lightning-rod, Sturdy",7,No
Mimikyu-disguised,778,Ghost,Fairy,Disguise Pokémon,0.2,0.7,Disguise,7,No
Bruxish,779,Water,Psychic,Gnash Teeth Pokémon,0.9,19.0,"Dazzling, Strong-jaw, Wonder-skin",7,No
Drampa,780,Normal,Dragon,Placid Pokémon,3.0,185.0,"Berserk, Sap-sipper, Cloud-nine",7,No
Dhelmise,781,Ghost,Grass,Sea Creeper Pokémon,3.9,210.0,Steelworker,7,No
Jangmo-o,782,Dragon,,Scaly Pokémon,0.6,29.7,"Bulletproof, Soundproof, Overcoat",7,No
Hakamo-o,783,Dragon,Fighting,Scaly Pokémon,1.2,47.0,"Bulletproof, Soundproof, Overcoat",7,No
Kommo-o,784,Dragon,Fighting,Scaly Pokémon,1.6,78.2,"Bulletproof, Soundproof, Overcoat",7,No
Tapu-koko,785,Electric,Fairy,Land Spirit Pokémon,1.8,20.5,"Electric-surge, Telepathy",7,Yes
Tapu-lele,786,Psychic,Fairy,Land Spirit Pokémon,1.2,18.6,"Psychic-surge, Telepathy",7,Yes
Tapu-bulu,787,Grass,Fairy,Land Spirit Pokémon,1.9,45.5,"Grassy-surge, Telepathy",7,Yes
Tapu-fini,788,Water,Fairy,Land Spirit Pokémon,1.3,21.2,"Misty-surge, Telepathy",7,Yes
Cosmog,789,Psychic,,Nebula Pokémon,0.2,0.1,Unaware,7,Yes
Cosmoem,790,Psychic,,Protostar Pokémon,0.1,999.9,Sturdy,7,Yes
Solgaleo,791,Psychic,Steel,Sunne Pokémon,3.4,230.0,Full-metal-body,7,Yes
Lunala,792,Psychic,Ghost,Moone Pokémon,4.0,120.0,Shadow-shield,7,Yes
Nihilego,793,Rock,Poison,Parasite Pokémon,1.2,55.5,Beast-boost,7,No
Buzzwole,794,Bug,Fighting,Swollen Pokémon,2.4,333.6,Beast-boost,7,No
Pheromosa,795,Bug,Fighting,Lissome Pokémon,1.8,25.0,Beast-boost,7,No
Xurkitree,796,Electric,,Glowing Pokémon,3.8,100.0,Beast-boost,7,No
Celesteela,797,Steel,Flying,Launch Pokémon,9.2,999.9,Beast-boost,7,No
Kartana,798,Grass,Steel,Drawn Sword Pokémon,0.3,0.1,Beast-boost,7,No
Guzzlord,799,Dark,Dragon,Junkivore Pokémon,5.5,888.0,Beast-boost,7,No
Necrozma,800,Psychic,,Prism Pokémon,2.4,230.0,Prism-armor,7,Yes
Magearna,801,Steel,Fairy,Artificial Pokémon,1.0,80.5,Soul-heart,7,Yes
Marshadow,802,Fighting,Ghost,Gloomdweller Pokémon,0.7,22.2,Technician,7,Yes
Poipole,803,Poison,,Poison Pin Pokémon,0.6,1.8,Beast-boost,7,No
Naganadel,804,Poison,Dragon,Poison Pin Pokémon,3.6,150.0,Beast-boost,7,No
Stakataka,805,Rock,Steel,Rampart Pokémon,5.5,820.0,Beast-boost,7,No
Blacephalon,806,Fire,Ghost,Fireworks Pokémon,1.8,13.0,Beast-boost,7,No
Zeraora,807,Electric,,Thunderclap Pokémon,1.5,44.5,Volt-absorb,7,Yes
Meltan,808,Steel,,Hex Nut Pokémon,0.2,8.0,Magnet-pull,7,Yes
Melmetal,809,Steel,,Hex Nut Pokémon,2.5,800.0,Iron-fist,7,Yes
Grookey,810,Grass,,Chimp Pokémon,0.3,5.0,"Overgrow, Grassy-surge",8,No
Thwackey,811,Grass,,Beat Pokémon,0.7,14.0,"Overgrow, Grassy-surge",8,No
Rillaboom,812,Grass,,Drummer Pokémon,2.1,90.0,"Overgrow, Grassy-surge",8,No
Scorbunny,813,Fire,,Rabbit Pokémon,0.3,4.5,"Blaze, Libero",8,No
Raboot,814,Fire,,Rabbit Pokémon,0.6,9.0,"Blaze, Libero",8,No
Cinderace,815,Fire,,Striker Pokémon,1.4,33.0,"Blaze, Libero",8,No
Sobble,816,Water,,Water Lizard Pokémon,0.3,4.0,"Torrent, Sniper",8,No
Drizzile,817,Water,,Water Lizard Pokémon,0.7,11.5,"Torrent, Sniper",8,No
Inteleon,818,Water,,Secret Agent Pokémon,1.9,45.2,"Torrent, Sniper",8,No
Skwovet,819,Normal,,Cheeky Pokémon,0.3,2.5,"Cheek-pouch, Gluttony",8,No
Greedent,820,Normal,,Greedy Pokémon,0.6,6.0,"Cheek-pouch, Gluttony",8,No
Rookidee,821,Flying,,Tiny Bird Pokémon,0.2,1.8,"Keen-eye, Unnerve, Big-pecks",8,No
Corvisquire,822,Flying,,Raven Pokémon,0.8,16.0,"Keen-eye, Unnerve, Big-pecks",8,No
Corviknight,823,Flying,Steel,Raven Pokémon,2.2,75.0,"Pressure, Unnerve, Mirror-armor",8,No
Blipbug,824,Bug,,Larva Pokémon,0.4,8.0,"Swarm, Compound-eyes, Telepathy",8,No
Dottler,825,Bug,Psychic,Radome Pokémon,0.4,19.5,"Swarm, Compound-eyes, Telepathy",8,No
Orbeetle,826,Bug,Psychic,Seven Spot Pokémon,0.4,40.8,"Swarm, Frisk, Telepathy",8,No
Nickit,827,Dark,,Fox Pokémon,0.6,8.9,"Run-away, Unburden, Stakeout",8,No
Thievul,828,Dark,,Fox Pokémon,1.2,19.9,"Run-away, Unburden, Stakeout",8,No
Gossifleur,829,Grass,,Flowering Pokémon,0.4,2.2,"Cotton-down, Regenerator, Effect-spore",8,No
Eldegoss,830,Grass,,Cotton Bloom Pokémon,0.5,2.5,"Cotton-down, Regenerator, Effect-spore",8,No
Wooloo,831,Normal,,Sheep Pokémon,0.6,6.0,"Fluffy, Run-away, Bulletproof",8,No
Dubwool,832,Normal,,Sheep Pokémon,1.3,43.0,"Fluffy, Steadfast, Bulletproof",8,No
Chewtle,833,Water,,Snapping Pokémon,0.3,8.5,"Strong-jaw, Shell-armor, Swift-swim",8,No
Drednaw,834,Water,Rock,Bite Pokémon,1.0,115.5,"Strong-jaw, Shell-armor, Swift-swim",8,No
Yamper,835,Electric,,Puppy Pokémon,0.3,13.5,"Ball-fetch, Rattled",8,No
Boltund,836,Electric,,Dog Pokémon,1.0,34.0,"Strong-jaw, Competitive",8,No
Rolycoly,837,Rock,,Coal Pokémon,0.3,12.0,"Steam-engine, Heatproof, Flash-fire",8,No
Carkol,838,Rock,Fire,Coal Pokémon,1.1,78.0,"Steam-engine, Flame-body, Flash-fire",8,No
Coalossal,839,Rock,Fire,Coal Pokémon,2.8,310.5,"Steam-engine, Flame-body, Flash-fire",8,No
Applin,840,Grass,Dragon,Apple Core Pokémon,0.2,0.5,"Ripen, Gluttony, Bulletproof",8,No
Flapple,841,Grass,Dragon,Apple Wing Pokémon,0.3,1.0,"Ripen, Gluttony, Hustle",8,No
Appletun,842,Grass,Dragon,Apple Nectar Pokémon,0.4,13.0,"Ripen, Gluttony, Thick-fat",8,No
Silicobra,843,Ground,,Sand Snake Pokémon,2.2,7.6,"Sand-spit, Shed-skin, Sand-veil",8,No
Sandaconda,844,Ground,,Sand Snake Pokémon,3.8,65.5,"Sand-spit, Shed-skin, Sand-veil",8,No
Cramorant,845,Flying,Water,Gulp Pokémon,0.8,18.0,Gulp-missile,8,No
Arrokuda,846,Water,,Rush Pokémon,0.5,1.0,"Swift-swim, Propeller-tail",8,No
Barraskewda,847,Water,,Skewer Pokémon,1.3,30.0,"Swift-swim, Propeller-tail",8,No
Toxel,848,Electric,Poison,Baby Pokémon,0.4,11.0,"Rattled, Static, Klutz",8,No
Toxtricity-amped,849,Electric,Poison,Punk Pokémon,1.6,40.0,"Punk-rock, Plus, Technician",8,No
Sizzlipede,850,Fire,Bug,Radiator Pokémon,0.7,1.0,"Flash-fire, White-smoke, Flame-body",8,No
Centiskorch,851,Fire,Bug,Radiator Pokémon,3.0,120.0,"Flash-fire, White-smoke, Flame-body",8,No
Clobbopus,852,Fighting,,Tantrum Pokémon,0.6,4.0,"Limber, Technician",8,No
Grapploct,853,Fighting,,Jujitsu Pokémon,1.6,39.0,"Limber, Technician",8,No
Sinistea,854,Ghost,,Black Tea Pokémon,0.1,0.2,"Weak-armor, Cursed-body",8,No
Polteageist,855,Ghost,,Black Tea Pokémon,0.2,0.4,"Weak-armor, Cursed-body",8,No
Hatenna,856,Psychic,,Calm Pokémon,0.4,3.4,"Healer, Anticipation, Magic-bounce",8,No
Hattrem,857,Psychic,,Serene Pokémon,0.6,4.8,"Healer, Anticipation, Magic-bounce",8,No
Hatterene,858,Psychic,Fairy,Silent Pokémon,2.1,5.1,"Healer, Anticipation, Magic-bounce",8,No
Impidimp,859,Dark,Fairy,Wily Pokémon,0.4,5.5,"Prankster, Frisk, Pickpocket",8,No
Morgrem,860,Dark,Fairy,Devious Pokémon,0.8,12.5,"Prankster, Frisk, Pickpocket",8,No
Grimmsnarl,861,Dark,Fairy,Bulk Up Pokémon,1.5,61.0,"Prankster, Frisk, Pickpocket",8,No
Obstagoon,862,Dark,Normal,Blocking Pokémon,1.6,46.0,"Reckless, Guts, Defiant",8,No
Perrserker,863,Steel,,Viking Pokémon,0.8,28.0,"Battle-armor, Tough-claws, Steely-spirit",8,No
Cursola,864,Ghost,,Coral Pokémon,1.0,0.4,"Weak-armor, Perish-body",8,No
Sirfetchd,865,Fighting,,Wild Duck Pokémon,0.8,117.0,"Steadfast, Scrappy",8,No
Mr-rime,866,Ice,Psychic,Comedian Pokémon,1.5,58.2,"Tangled-feet, Screen-cleaner, Ice-body",8,No
Runerigus,867,Ground,Ghost,Grudge Pokémon,1.6,66.6,Wandering-spirit,8,No
Milcery,868,Fairy,,Cream Pokémon,0.2,0.3,"Sweet-veil, Aroma-veil",8,No
Alcremie,869,Fairy,,Cream Pokémon,0.3,0.5,"Sweet-veil, Aroma-veil",8,No
Falinks,870,Fighting,,Formation Pokémon,3.0,62.0,"Battle-armor, Defiant",8,No
Pincurchin,871,Electric,,Sea Urchin Pokémon,0.3,1.0,"Lightning-rod, Electric-surge",8,No
Snom,872,Ice,Bug,Worm Pokémon,0.3,3.8,"Shield-dust, Ice-scales",8,No
Frosmoth,873,Ice,Bug,Frost Moth Pokémon,1.3,42.0,"Shield-dust, Ice-scales",8,No
Stonjourner,874,Rock,,Big Rock Pokémon,2.5,520.0,Power-spot,8,No
Eiscue-ice,875,Ice,,Penguin Pokémon,1.4,89.0,Ice-face,8,No
Indeedee-male,876,Psychic,Normal,Emotion Pokémon,0.9,28.0,"Inner-focus, Synchronize, Psychic-surge",8,No
Morpeko-full-belly,877,Electric,Dark,Two-Sided Pokémon,0.3,3.0,Hunger-switch,8,No
Cufant,878,Steel,,Copperderm Pokémon,1.2,100.0,"Sheer-force, Heavy-metal",8,No
Copperajah,879,Steel,,Copperderm Pokémon,3.0,650.0,"Sheer-force, Heavy-metal",8,No
Dracozolt,880,Electric,Dragon,Fossil Pokémon,1.8,190.0,"Volt-absorb, Hustle, Sand-rush",8,No
Arctozolt,881,Electric,Ice,Fossil Pokémon,2.3,150.0,"Volt-absorb, Static, Slush-rush",8,No
Dracovish,882,Water,Dragon,Fossil Pokémon,2.3,215.0,"Water-absorb, Strong-jaw, Sand-rush",8,No
Arctovish,883,Water,Ice,Fossil Pokémon,2.0,175.0,"Water-absorb, Ice-body, Slush-rush",8,No
Duraludon,884,Steel,Dragon,Alloy Pokémon,1.8,40.0,"Light-metal, Heavy-metal, Stalwart",8,No
Dreepy,885,Dragon,Ghost,Lingering Pokémon,0.5,2.0,"Clear-body, Infiltrator, Cursed-body",8,No
Drakloak,886,Dragon,Ghost,Caretaker Pokémon,1.4,11.0,"Clear-body, Infiltrator, Cursed-body",8,No
Dragapult,887,Dragon,Ghost,Stealth Pokémon,3.0,50.0,"Clear-body, Infiltrator, Cursed-body",8,No
Zacian,888,Fairy,,Warrior Pokémon,2.8,110.0,Intrepid-sword,8,Yes
Zamazenta,889,Fighting,,Warrior Pokémon,2.9,210.0,Dauntless-shield,8,Yes
Eternatus,890,Poison,Dragon,Gigantic Pokémon,20.0,950.0,Pressure,8,Yes
Kubfu,891,Fighting,,Wushu Pokémon,0.6,12.0,Inner-focus,8,Yes
Urshifu-single-strike,892,Fighting,Dark,Wushu Pokémon,1.9,105.0,Unseen-fist,8,Yes
Zarude,893,Dark,Grass,Rogue Monkey Pokémon,1.8,70.0,Leaf-guard,8,Yes
Regieleki,894,Electric,,Electron Pokémon,1.2,145.0,Transistor,8,Yes
Regidrago,895,Dragon,,Dragon Orb Pokémon,2.1,200.0,Dragons-maw,8,Yes
Glastrier,896,Ice,,Wild Horse Pokémon,2.2,800.0,Chilling-neigh,8,Yes
Spectrier,897,Ghost,,Swift Horse Pokémon,2.0,44.5,Grim-neigh,8,Yes
Calyrex,898,Psychic,Grass,King Pokémon,1.1,7.7,Unnerve,8,Yes
Wyrdeer,899,Normal,Psychic,おおツノポケモン,1.8,95.1,"Intimidate, Frisk, Sap-sipper",8,No
Kleavor,900,Bug,Rock,まさかりポケモン,1.8,89.0,"Swarm, Sheer-force, Sharpness",8,No
Ursaluna,901,Ground,Normal,でいたんポケモン,2.4,290.0,"Guts, Bulletproof, Unnerve",8,No
Basculegion-male,902,Water,Ghost,おおうおポケモン,3.0,110.0,"Swift-swim, Adaptability, Mold-breaker",8,No
Sneasler,903,Fighting,Poison,クライミングポケモン,1.3,43.0,"Pressure, Unburden, Poison-touch",8,No
Overqwil,904,Dark,Poison,けんざんポケモン,2.5,60.5,"Poison-point, Swift-swim, Intimidate",8,No
Enamorus-incarnate,905,Fairy,Flying,あいぞうポケモン,1.6,48.0,"Cute-charm, Contrary",8,Yes
```
</details>

- queries.sql

```sql
{{ define "create" }}
    CREATE TABLE IF NOT EXISTS types (
        id INTEGER PRIMARY KEY,
        name TEXT UNIQUE NOT NULL
    );

    CREATE TABLE IF NOT EXISTS classifications (
        id INTEGER PRIMARY KEY,
        name TEXT UNIQUE NOT NULL
    );

    CREATE TABLE IF NOT EXISTS abilities (
        id INTEGER PRIMARY KEY,
        name TEXT UNIQUE NOT NULL
    );

    CREATE TABLE IF NOT EXISTS pokemons (
        number INTEGER PRIMARY KEY,
        name TEXT UNIQUE,
        height NUMERIC,
        weight NUMERIC,
        generation INTEGER,
        legendary BOOLEAN
    );

    CREATE TABLE IF NOT EXISTS pokemon_types (
        pokemon_number INTEGER REFERENCES pokemons (number),
        type_id INTEGER REFERENCES types (id)
    );

    CREATE TABLE IF NOT EXISTS pokemon_classifications (
        pokemon_number INTEGER REFERENCES pokemons (number),
        classification_id INTEGER REFERENCES classifications (id)
    );

    CREATE TABLE IF NOT EXISTS pokemon_abilities (
        pokemon_number INTEGER REFERENCES pokemons (number),
        ability_id INTEGER REFERENCES abilities (id)
    );
{{ end }}

{{ define "insert_types" }}
    INSERT INTO types (name) VALUES
    {{ $first := true }}
    {{ range . }}
        {{ with (index . 2) }}
            {{ if not $first }}, {{ end }}
            ({{ . }})
            {{ $first = false }}
        {{ end }}

        {{ with (index . 3) }}
            {{ if not $first }}, {{ end }}
            ({{ . }})
            {{ $first = false }}
        {{ end }}
    {{ end }}
    ON CONFLICT DO NOTHING;
{{ end }}

{{ define "insert_classifications" }}
    INSERT INTO classifications (name) VALUES 
        {{ range $i, $p := . }}
            {{ if $i }}, {{ end }}
            ({{ index $p 4 }})
        {{ end }}
    ON CONFLICT DO NOTHING;
{{ end }}

{{ define "insert_abilities" }}
    INSERT INTO abilities (name) VALUES
    {{ $first := true }}
    {{ range . }}
        {{ range (splitList ", " (index . 7)) }}
            {{ if not $first }}, {{ end }}
            ({{ . }})
            {{ $first = false }}
        {{ end }}
    {{ end }}
    ON CONFLICT DO NOTHING;
{{ end }}

{{ define "insert_pokemons" }}
    INSERT INTO pokemons (number, name, height, weight, generation, legendary) VALUES
    {{ range $i, $p := . }}
        {{ if $i }}, {{ end }}
        (
            {{ atoi (index $p 1) }}
            , {{ index $p 0 }}
            , {{ float64 (index $p 5) }}
            , {{ float64 (index $p 6) }}
            , {{ atoi (index $p 8) }}
            , {{ eq (index $p 9) "Yes" }}
        )
    {{ end }};
{{ end }}

{{ define "insert_pokemon_types" }}
    INSERT INTO pokemon_types (pokemon_number, type_id) VALUES
    {{ $first := true }}
    {{ range . }}
        {{ $number := atoi (index . 1) }}

        {{ if (index . 2) }}
            {{ if not $first }},{{ end }}
            ({{ $number }}, (SELECT id FROM types WHERE name = {{ index . 2 }}))
            {{ $first = false }}
        {{ end }}

        {{ if (index . 3) }}
            {{ if not $first }},{{ end }}
            ({{ $number }}, (SELECT id FROM types WHERE name = {{ index . 3 }}))
            {{ $first = false }}
        {{ end }}
    {{ end }};
{{ end }}

{{ define "insert_pokemon_classifications" }}
    INSERT INTO pokemon_classifications (pokemon_number, classification_id) VALUES
    {{ range $i, $p := . }}
        {{ if $i }}, {{ end }}
        ({{ atoi (index $p 1) }}, (SELECT id FROM classifications WHERE name = {{ index $p 4 }}))
    {{ end }};
{{ end }}

{{ define "insert_pokemon_abilities" }}
    INSERT INTO pokemon_abilities (pokemon_number, ability_id) VALUES
    {{ $first := true }}
    {{ range $p := . }}
        {{ range (splitList ", " (index $p 7)) }}
            {{ if not $first }},{{ end }}
            ({{ atoi (index $p 1) }}, (SELECT id FROM abilities WHERE name = {{ . }}))
            {{ $first = false }}
        {{ end }}
    {{ end }};
{{ end }}

{{ define "query" }}
    SELECT 
        p.number, 			{{ Scan "Number" }}	
        p.name, 			{{ Scan "Name" }}
        p.height, 			{{ Scan "Height" }}	
        p.weight, 			{{ Scan "Weight" }}
        p.generation, 			{{ Scan "Generation" }}
        p.legendary, 			{{ Scan "Legendary" }}
        IFNULL(pt.type_names, ''), 	{{ ScanSplit "Types" "," }}
        c.name AS classification, 	{{ Scan "Classification" }}
        IFNULL(pa.ability_names, '')	{{ ScanSplit "Abilities" "," }}
    FROM pokemons p
    LEFT JOIN (
        SELECT pokemon_number, GROUP_CONCAT(types.name, ',') AS type_names
        FROM pokemon_types 
        JOIN types ON types.id = pokemon_types.type_id
        GROUP BY pokemon_number
    ) pt ON p.number = pt.pokemon_number
    LEFT JOIN (
        SELECT pokemon_number, GROUP_CONCAT(abilities.name, ',') AS ability_names
        FROM pokemon_abilities 
        JOIN abilities ON abilities.id = pokemon_abilities.ability_id
        GROUP BY pokemon_number
    ) pa ON p.number = pa.pokemon_number
    LEFT JOIN pokemon_classifications pc ON p.number = pc.pokemon_number
    LEFT JOIN classifications c ON c.id = pc.classification_id
    WHERE 1=1
    {{ if .HeightRange }}
        AND p.height >= {{ index .HeightRange 0 }} AND p.height <= {{ index .HeightRange 1 }}
    {{ end }}
    {{ if .WeightRange }}
        AND p.weight >= {{ index .WeightRange 0 }} AND p.weight <= {{ index .WeightRange 1 }}
    {{ end }}
    {{ if .Generation }}
        AND p.generation = {{ .Generation }}
    {{ end }}
    {{ if .TypeOneOf }}
        AND p.number IN (
            SELECT pokemon_number 
            FROM pokemon_types 
            JOIN types ON types.id = pokemon_types.type_id 
            WHERE types.name IN (
                {{ range $i, $t := .TypeOneOf }}
                    {{ if $i }}, {{ end }}
                    {{ $t }}
                {{ end }}
            )
        )
    {{ end }}
    {{ if .Classification }}
        AND p.classification = {{ .Classification }}
    {{ end }}
    {{ if .AbilityOneOf }}
        AND p.number IN (
            SELECT pokemon_number 
            FROM pokemon_abilities 
            JOIN abilities ON abilities.id = pokemon_abilities.ability_id 
            WHERE abilities.name IN (
                {{ range $i, $a := .AbilityOneOf }}
                    {{ if $i }}, {{ end }}
                    {{ $a }}
                {{ end }}
            )
        )
    {{ end }}
    ORDER BY p.number;
{{ end }}
```

- Example with sqlite

```go
package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Masterminds/sprig/v3"
	"github.com/wroge/sqlt"

	_ "modernc.org/sqlite"
)

type Pokemon struct {
	Number         int      `json:"number"`
	Name           string   `json:"name"`
	Height         float64  `json:"height"`
	Weight         float64  `json:"weight"`
	Generation     int      `json:"generation"`
	Legendary      bool     `json:"legendary"`
	Types          []string `json:"types"`
	Classification string   `json:"classification"`
	Abilities      []string `json:"abilities"`
}

func NewPointer[T any](t T) Pointer[T] {
	return &t
}

type Pointer[T any] *T

type Query struct {
	HeightRange    Pointer[[2]float64]
	WeightRange    Pointer[[2]float64]
	Generation     Pointer[int]
	Legendary      Pointer[bool]
	TypeOneOf      Pointer[[]string]
	Classification Pointer[string]
	AbilityOneOf   Pointer[[]string]
}

var (
	config = sqlt.Config{
		Placeholder: sqlt.Question,
		Cache:       &sqlt.Cache{},
		Templates: []sqlt.Template{
			sqlt.Funcs(sprig.TxtFuncMap()),
			sqlt.ParseFiles("./queries.sql"),
		},
	}

	create = sqlt.Exec[any](config, sqlt.Lookup("create"))

	insert = sqlt.Transaction(
		nil,
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_types")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_classifications")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_abilities")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemons")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemon_types")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemon_classifications")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemon_abilities")),
	)

	query = sqlt.All[Query, Pokemon](config, sqlt.Lookup("query"))
)

func main() {
	db, err := sql.Open("sqlite", "file:pokemon.db?mode=memory") // ?mode=memory
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	file, err := os.Open("./pokemon_data_pokeapi.csv")
	if err != nil {
		panic(err)
	}

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		panic(err)
	}

	_, err = create.Exec(ctx, db, nil)
	if err != nil {
		panic(err)
	}

	_, err = insert.Exec(ctx, db, records[1:])
	if err != nil {
		panic(err)
	}

	pokemons, err := query.Exec(ctx, db, Query{
		TypeOneOf:  NewPointer([]string{"Dragon"}),
		Generation: NewPointer(1),
	})
	if err != nil {
		panic(err)
	}

	js, err := json.MarshalIndent(pokemons, "", "   ")
	if err != nil {
		panic(err)
	}

	fmt.Println(string(js))
	// [
	//	   {
	//	      "number": 147,
	//	      "name": "Dratini",
	//	      "height": 1.8,
	//	      "weight": 3.3,
	//	      "generation": 1,
	//	      "legendary": false,
	//	      "types": [
	//	         "Dragon"
	//	      ],
	//	      "classification": "Dragon Pokémon",
	//	      "abilities": [
	//	         "Shed-skin",
	//	         "Marvel-scale"
	//	      ]
	//	   },
	//	   {
	//	      "number": 148,
	//	      "name": "Dragonair",
	//	      "height": 4,
	//	      "weight": 16.5,
	//	      "generation": 1,
	//	      "legendary": false,
	//	      "types": [
	//	         "Dragon"
	//	      ],
	//	      "classification": "Dragon Pokémon",
	//	      "abilities": [
	//	         "Shed-skin",
	//	         "Marvel-scale"
	//	      ]
	//	   },
	//	   {
	//	      "number": 149,
	//	      "name": "Dragonite",
	//	      "height": 2.2,
	//	      "weight": 210,
	//	      "generation": 1,
	//	      "legendary": false,
	//	      "types": [
	//	         "Dragon",
	//	         "Flying"
	//	      ],
	//	      "classification": "Dragon Pokémon",
	//	      "abilities": [
	//	         "Inner-focus",
	//	         "Multiscale"
	//	      ]
	//	   }
	//
	// ]
}
```

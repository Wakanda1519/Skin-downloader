package approved

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Skin struct {
	WorkshopID uint64
	Name       string
	ItemName   string
}

var approvedRe = regexp.MustCompile(`new\s+ApprovedSkinInfo\(\s*(\d+)\s*u?L?\s*,\s*"((?:[^"\\]|\\.)*)"\s*,\s*"((?:[^"\\]|\\.)*)"\s*,\s*"((?:[^"\\]|\\.)*)"\s*\)`)

func Parse(path string) ([]Skin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	matches := approvedRe.FindAllStringSubmatch(string(data), -1)
	skins := make([]Skin, 0, len(matches))
	for _, m := range matches {
		id, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			continue
		}
		skins = append(skins, Skin{
			WorkshopID: id,
			Name:       unescapeCS(m[2]),
			ItemName:   unescapeCS(m[4]),
		})
	}
	return skins, nil
}

func unescapeCS(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

type ItemGroup struct {
	Name string
	IDs  []uint64
}

var shortNames = map[string]string{
	"garage door":           "wall.frame.garagedoor",
	"semi-automatic rifle":  "rifle.semiauto",
	"coffee can helmet":     "coffeecan.helmet",
	"roadsign vest":         "roadsign.jacket",
	"roadsign pants":        "roadsign.kilt",
	"pants":                 "pants",
	"hoodie":                "hoodie",
	"riot helmet":           "riot.helmet",
	"crossbow":              "crossbow",
	"python":                "pistol.python",
	"sheet metal door":      "door.metal",
	"ak47":                  "rifle.ak",
	"sleeping bag":          "sleepingbag",
	"furnace":               "furnace",
	"custom smg":            "smg.2",
	"bolt rifle":            "rifle.bolt",
	"water purifier":        "water.purifier",
	"large wood box":        "box.wooden.large",
	"hunting bow":           "bow.hunting",
	"stone pick axe":        "pickaxe.stone",
	"stone hatchet":         "hatchet.stone",
	"revolver":              "pistol.revolver",
	"vending machine":       "vending.machine",
	"hatchet":               "hatchet",
	"pick axe":              "pickaxe",
	"thompson":              "smg.thompson",
	"metal chest plate":     "metal.plate.torso",
	"metal facemask":        "metal.facemask",
	"cap":                   "cap",
	"boonie hat":            "boonie.hat",
	"burlap pants":          "burlap.pants",
	"burlap shirt":          "burlap.shirt",
	"fridge":                "fridge",
	"pump shotgun":          "shotgun.pump",
	"combat knife":          "knife.combat",
	"lr300":                 "rifle.lr300",
	"rock":                  "rock",
	"rug":                   "rug",
	"m39":                   "rifle.m39",
	"wooden door":           "door.wood",
	"m249":                  "lmg.m249",
	"semi-automatic pistol": "pistol.semiauto",
	"hammer":                "hammer",
	"satchel charge":        "explosive.satchel",
	"bandana":               "bandana",
	"burlap headwrap":       "burlap.headwrap",
	"mp5":                   "smg.mp5",
	"waterpipe shotgun":     "shotgun.waterpipe",
	"armored door":          "door.metal.armored",
	"double barrel shotgun": "shotgun.double",
	"wood storage box":      "box.wooden",
	"rocket launcher":       "rocket.launcher",
	"sword":                 "sword",
	"work boots":            "shoes.boots",
	"leather gloves":        "gloves.leather",
	"bone club":             "bone.club",
	"long tshirt":           "tshirt.long",
	"shorts":                "shorts",
	"tshirt":                "tshirt",
	"eoka pistol":           "pistol.eoka",
	"locker":                "locker",
	"hide pants":            "attire.hide.pants",
	"hide shirt":            "attire.hide.vest",
	"hide poncho":           "attire.hide.poncho",
	"vagabond jacket":       "jacket.vagabond",
	"salvaged icepick":      "icepick.salvaged",
	"bone knife":            "knife.bone",
	"chair":                 "chair",
	"hide shoes":            "attire.hide.boots",
	"longsword":             "longsword",
	"bucket helmet":         "bucket.helmet",
	"balaclava":             "balaclava",
	"f1 grenade":            "grenade.f1",
	"concrete barricade":    "barricade.concrete",
	"snow jacket":           "jacket.snow",
	"table":                 "table",
	"burlap shoes":          "burlap.shoes",
	"miner hat":             "hat.miner",
	"collared shirt":        "shirt.collared",
	"deer skull mask":       "deer.skull.mask",
	"bearskin rug":          "rug.bear",
	"acoustic guitar":       "guitar.acoustic",
	"hide halterneck":       "attire.hide.halterneck",
	"hide skirt":            "attire.hide.skirt",
	"reactive target":       "target.reactive",
	"sandbag barricade":     "barricade.sandbags",
	"beenie hat":            "hat.beenie",
	"tank top":              "shirt.tanktop",
}

func ShortName(itemName string) string {
	if sn, ok := shortNames[strings.ToLower(itemName)]; ok {
		return sn
	}
	return itemName
}

func Group(skins []Skin) []ItemGroup {
	index := make(map[string]int, len(skins))
	groups := make([]ItemGroup, 0)
	for _, s := range skins {
		name := ShortName(s.ItemName)
		key := strings.ToLower(name)
		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, ItemGroup{Name: name})
		}
		groups[i].IDs = append(groups[i].IDs, s.WorkshopID)
	}
	return groups
}

func UniqueIDs(skins []Skin) []uint64 {
	seen := make(map[uint64]struct{}, len(skins))
	ids := make([]uint64, 0, len(skins))
	for _, s := range skins {
		if _, ok := seen[s.WorkshopID]; ok {
			continue
		}
		seen[s.WorkshopID] = struct{}{}
		ids = append(ids, s.WorkshopID)
	}
	return ids
}

func RemoveIDs(groups []ItemGroup, remove map[uint64]struct{}) []ItemGroup {
	out := make([]ItemGroup, 0, len(groups))
	for _, g := range groups {
		kept := make([]uint64, 0, len(g.IDs))
		for _, id := range g.IDs {
			if _, drop := remove[id]; drop {
				continue
			}
			kept = append(kept, id)
		}
		if len(kept) > 0 {
			out = append(out, ItemGroup{Name: g.Name, IDs: kept})
		}
	}
	return out
}

func WriteJSON(path string, groups []ItemGroup) error {
	var b strings.Builder
	b.WriteString("{\n")
	for gi, g := range groups {
		key, err := json.Marshal(g.Name)
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "  %s: [", key)
		if len(g.IDs) == 0 {
			b.WriteString("]")
		} else {
			b.WriteString("\n")
			for ii, id := range g.IDs {
				fmt.Fprintf(&b, "    %d", id)
				if ii < len(g.IDs)-1 {
					b.WriteString(",")
				}
				b.WriteString("\n")
			}
			b.WriteString("  ]")
		}
		if gi < len(groups)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

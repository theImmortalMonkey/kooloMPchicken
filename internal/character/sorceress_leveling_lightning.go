package character

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/difficulty"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
)

const (
	SorceressLevelingLightningMaxAttacksLoop = 10
)

type SorceressLevelingLightning struct {
	BaseCharacter
}

func (s SorceressLevelingLightning) CheckKeyBindings() []skill.ID {
	requireKeybindings := []skill.ID{skill.TomeOfTownPortal}
	missingKeybindings := []skill.ID{}

	for _, cskill := range requireKeybindings {
		if _, found := s.Data.KeyBindings.KeyBindingForSkill(cskill); !found {
			missingKeybindings = append(missingKeybindings, cskill)
		}
	}

	if len(missingKeybindings) > 0 {
		s.Logger.Debug("There are missing required key bindings.", slog.Any("Bindings", missingKeybindings))
	}

	return missingKeybindings
}

func (s SorceressLevelingLightning) KillMonsterSequence(
	monsterSelector func(d game.Data) (data.UnitID, bool),
	skipOnImmunities []stat.Resist,
) error {
	completedAttackLoops := 0
	previousUnitID := 0

	for {
		id, found := monsterSelector(*s.Data)
		if !found {
			return nil
		}
		if previousUnitID != int(id) {
			completedAttackLoops = 0
		}

		if !s.preBattleChecks(id, skipOnImmunities) {
			return nil
		}

		if completedAttackLoops >= SorceressLevelingLightningMaxAttacksLoop {
			return nil
		}

		monster, found := s.Data.Monsters.FindByID(id)
		if !found {
			s.Logger.Info("Monster not found", slog.String("monster", fmt.Sprintf("%v", monster)))
			return nil
		}

		lvl, _ := s.Data.PlayerUnit.FindStat(stat.Level, 0)
		if s.Data.PlayerUnit.MPPercent() < 15 && lvl.Value < 15 {
			s.Logger.Debug("Low mana, using primary attack")
			step.PrimaryAttack(id, 1, false, step.Distance(1, 3))
		} else {
			if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.Blizzard); found {
				if completedAttackLoops%2 == 0 {
					for _, m := range s.Data.Monsters.Enemies() {
						if d := s.PathFinder.DistanceFromMe(m.Position); d < 4 {
							s.Logger.Debug("Monster close, casting Blizzard")
							step.SecondaryAttack(skill.Blizzard, m.UnitID, 1, step.Distance(25, 30))
							break
						}
					}
				}

				s.Logger.Debug("Using Blizzard")

				step.SecondaryAttack(skill.Blizzard, id, 1, step.Distance(25, 30))
				step.PrimaryAttack(id, 3, false, step.Distance(25, 30))

			} else if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.Nova); found {
				s.Logger.Debug("Using Nova")
				step.SecondaryAttack(skill.Nova, id, 4, step.Distance(1, 5))
			} else if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.ChargedBolt); found {
				s.Logger.Debug("Using ChargedBolt")
				step.SecondaryAttack(skill.ChargedBolt, id, 4, step.Distance(1, 5))
			} else if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.FireBolt); found {
				s.Logger.Debug("Using FireBolt")
				step.SecondaryAttack(skill.FireBolt, id, 4, step.Distance(1, 5))
			} else {
				s.Logger.Debug("No secondary skills available, using primary attack")
				step.PrimaryAttack(id, 1, false, step.Distance(1, 3))
			}
		}

		completedAttackLoops++
		previousUnitID = int(id)
	}
}

func (s SorceressLevelingLightning) killMonster(npc npc.ID, t data.MonsterType) error {
	return s.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		m, found := d.Monsters.FindOne(npc, t)
		if !found {
			return 0, false
		}

		return m.UnitID, true
	}, nil)
}

func (s SorceressLevelingLightning) BuffSkills() []skill.ID {
	skillsList := make([]skill.ID, 0)
	if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.EnergyShield); found {
		skillsList = append(skillsList, skill.EnergyShield)
	}

	if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.ThunderStorm); found {
		skillsList = append(skillsList, skill.ThunderStorm)
	}

	armors := []skill.ID{skill.ChillingArmor, skill.ShiverArmor, skill.FrozenArmor}
	for _, armor := range armors {
		if _, found := s.Data.KeyBindings.KeyBindingForSkill(armor); found {
			skillsList = append(skillsList, armor)
			break
		}
	}

	return skillsList
}

func (s SorceressLevelingLightning) PreCTABuffSkills() []skill.ID {
	return []skill.ID{}
}

func (s SorceressLevelingLightning) staticFieldCasts() int {
	casts := 6
	ctx := context.Get()

	switch ctx.CharacterCfg.Game.Difficulty {
	case difficulty.Normal:
		casts = 8
	}
	s.Logger.Debug("Static Field casts", "count", casts)
	return casts
}

func (s SorceressLevelingLightning) ShouldResetSkills() bool {
	lvl, _ := s.Data.PlayerUnit.FindStat(stat.Level, 0)
	if lvl.Value >= 25 && s.Data.PlayerUnit.Skills[skill.Nova].Level > 10 {
		s.Logger.Info("Resetting skills: Level 25+ and Nova level > 10")
		return true
	}

	return false
}

func (s SorceressLevelingLightning) SkillsToBind() (skill.ID, []skill.ID) {
	level, _ := s.Data.PlayerUnit.FindStat(stat.Level, 0)
	skillBindings := []skill.ID{
		skill.TomeOfTownPortal,
	}

	// Add skills only if the character level is high enough
	if level.Value >= 4 {
		skillBindings = append(skillBindings, skill.FrozenArmor)
	}
	if level.Value >= 6 {
		skillBindings = append(skillBindings, skill.StaticField)
	}
	if level.Value >= 18 {
		skillBindings = append(skillBindings, skill.Teleport)
	}

	if s.Data.PlayerUnit.Skills[skill.Blizzard].Level > 0 {
		skillBindings = append(skillBindings, skill.Blizzard)
	} else if s.Data.PlayerUnit.Skills[skill.Nova].Level > 1 {
		skillBindings = append(skillBindings, skill.Nova)
	} else if s.Data.PlayerUnit.Skills[skill.ChargedBolt].Level > 0 {
		skillBindings = append(skillBindings, skill.ChargedBolt)
	} else if s.Data.PlayerUnit.Skills[skill.FireBolt].Level > 0 {
		skillBindings = append(skillBindings, skill.FireBolt)
	}

	mainSkill := skill.AttackSkill
	if s.Data.PlayerUnit.Skills[skill.GlacialSpike].Level > 0 {
		mainSkill = skill.GlacialSpike
	}

	s.Logger.Info("Skills bound", "mainSkill", mainSkill, "skillBindings", skillBindings)
	return mainSkill, skillBindings
}

func (s SorceressLevelingLightning) StatPoints() []context.StatAllocation {

	// Define target totals (including base stats)
	targets := []context.StatAllocation{
		{Stat: stat.Vitality, Points: 50},  // Base 10 + 40
		{Stat: stat.Strength, Points: 25},  // Base 10 + 15
		{Stat: stat.Vitality, Points: 65},  // Previous 50 + 15
		{Stat: stat.Strength, Points: 47},  // Previous 25 + 22
		{Stat: stat.Vitality, Points: 999}, // Rest into vit
	}

	return targets
}

func (s SorceressLevelingLightning) SkillPoints() []skill.ID {
	lvl, _ := s.Data.PlayerUnit.FindStat(stat.Level, 0)
	var skillsToLearnThisLevel []skill.ID // This slice will hold the skill(s) to learn at the current character level

	// --- Skill Plan for Levels 1-24 (Lightning Leveling Phase) ---
	// This map defines which skill to learn at each specific character level.
	// You typically get 1 skill point per level-up.
	// Example: level 2 gets ChargedBolt, level 3 gets ChargedBolt, level 4 gets FrozenArmor, etc.
	// IMPORTANT: Ensure skill prerequisites (level, prior skills) are met for the level you assign them.
	// Also, if you gain extra points from quests (e.g., Den of Evil, Radament, Izual),
	// you'll need to decide if you want to explicitly map those levels too, or if they
	// should naturally fall into the next unlearned skill in your progression.
	levelUpSkillPlan := map[int]skill.ID{
		2:  skill.ChargedBolt,
		3:  skill.ChargedBolt,
		4:  skill.FrozenArmor,   // Example: First utility skill
		5:  skill.ChargedBolt,
		6:  skill.StaticField,   // Example: Early damage utility
		7:  skill.ChargedBolt,
		8:  skill.ChargedBolt,
		9:  skill.ChargedBolt,
		10: skill.Telekinesis,   // Prerequisite for Teleport
		11: skill.Warmth,        // Mana regeneration
		12: skill.Nova,          // Your primary lightning skill
		13: skill.ChargedBolt,   // Continue boosting Charged Bolt or Nova
		14: skill.Nova,
		15: skill.ChargedBolt,
		16: skill.Nova,
		17: skill.StaticField,   // More points in Static Field if desired
		18: skill.Teleport,      // Mobility skill (requires Telekinesis)
		19: skill.Nova,
		20: skill.ChargedBolt,
		21: skill.Nova,
		22: skill.Nova,
		23: skill.Nova,
		24: skill.Nova,
		// Add more levels and skills as needed up to Level 24 if your plan goes higher
		// before the respec at Level 25.
	}

	// --- Logic for Skill Point Allocation ---
	if lvl.Value < 25 {
		// During the leveling phase (before Level 25 respec)
		if sID, ok := levelUpSkillPlan[lvl.Value]; ok {
			// Check if we have a plan for the current character level

			// Get the current level of the skill in-game
			currentSkillLevel := 0
			if charSkill, found := s.Data.PlayerUnit.Skills[sID]; found {
				currentSkillLevel = int(charSkill.Level)
			}

			// Add the skill to the list if it hasn't reached its max hard points yet (usually 20)
			// This prevents trying to put points into an already maxed skill.
			// You might want a more sophisticated check here if you allow +skills from gear.
			if currentSkillLevel < 20 { // 20 is typically the max hard points for a skill
				skillsToLearnThisLevel = append(skillsToLearnThisLevel, sID)
			} else {
				// If the skill for this level is already maxed, log it and try to find the next skill
				// (This part requires more advanced logic if you want to "skip" a level's planned skill
				// and allocate the point to the next available one. For this simpler approach,
				// if the designated skill is maxed, no point is allocated at this level via the plan.)
				s.Logger.Debug("Skill already maxed for current level's plan", "skill", skill.SkillNames[sID], "level", lvl.Value)
			}
		}
	} else {
		// --- Skill Plan for Levels 25+ (After Skill Reset / Final Build Phase) ---
		// For the post-Level 25 phase, where you likely respec and have many points to dump,
		// an ordered list of all skills to max out is often more practical than level-by-level.
		// The `action` package will then iterate through this list until all `remainingPoints` are spent.
		skillsToLearnThisLevel = []skill.ID{
			skill.StaticField, // Prioritize key skills for the final build
			skill.StaticField,
			skill.StaticField,
			skill.StaticField,
			skill.Telekinesis,
			skill.Teleport,
			skill.FrozenArmor, // Or Shiver Armor/Chilling Armor depending on preference
			skill.IceBolt,
			skill.IceBlast,
			skill.FrostNova,
			skill.GlacialSpike,
			skill.Blizzard, // Start maxing Blizzard
			skill.Blizzard,
			skill.Warmth,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.IceBlast, // Synergies for Blizzard
			skill.IceBlast,
			skill.IceBlast,
			skill.IceBlast,
			skill.IceBlast,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.ColdMastery, // Essential for cold damage
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.Blizzard,
			skill.ColdMastery,
			skill.ColdMastery,
			skill.ColdMastery,
			skill.ColdMastery,
			skill.GlacialSpike, // More synergies
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			skill.GlacialSpike,
			// Continue with more Cold Mastery, Ice Blast, Frost Nova as desired to max synergies
		}
	}

	s.Logger.Info("Deciding skill point allocation", "level", lvl.Value, "skillsToLearnThisLevel", skillsToLearnThisLevel)
	return skillsToLearnThisLevel
}

func (s SorceressLevelingLightning) KillCountess() error {
	return s.killMonster(npc.DarkStalker, data.MonsterTypeSuperUnique)
}

func (s SorceressLevelingLightning) KillAndariel() error {
	m, _ := s.Data.Monsters.FindOne(npc.Andariel, data.MonsterTypeNone)
	_ = step.SecondaryAttack(skill.StaticField, m.UnitID, s.staticFieldCasts(), step.Distance(3, 5))
	return s.killMonster(npc.Andariel, data.MonsterTypeNone)
}

func (s SorceressLevelingLightning) KillSummoner() error {
	return s.killMonster(npc.Summoner, data.MonsterTypeNone)
}

func (s SorceressLevelingLightning) KillDuriel() error {
	m, _ := s.Data.Monsters.FindOne(npc.Duriel, data.MonsterTypeUnique)
	_ = step.SecondaryAttack(skill.StaticField, m.UnitID, s.staticFieldCasts(), step.Distance(1, 5))

	return s.killMonster(npc.Duriel, data.MonsterTypeUnique)
}

func (s SorceressLevelingLightning) KillCouncil() error {
	return s.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		// Exclude monsters that are not council members
		var councilMembers []data.Monster
		for _, m := range d.Monsters {
			if m.Name == npc.CouncilMember || m.Name == npc.CouncilMember2 || m.Name == npc.CouncilMember3 {
				councilMembers = append(councilMembers, m)
			}
		}

		// Order council members by distance
		sort.Slice(councilMembers, func(i, j int) bool {
			distanceI := s.PathFinder.DistanceFromMe(councilMembers[i].Position)
			distanceJ := s.PathFinder.DistanceFromMe(councilMembers[j].Position)

			return distanceI < distanceJ
		})

		for _, m := range councilMembers {
			return m.UnitID, true
		}

		return 0, false
	}, nil)
}

func (s SorceressLevelingLightning) KillMephisto() error {
	m, _ := s.Data.Monsters.FindOne(npc.Mephisto, data.MonsterTypeNone)
	_ = step.SecondaryAttack(skill.StaticField, m.UnitID, s.staticFieldCasts(), step.Distance(1, 5))
	return s.killMonster(npc.Mephisto, data.MonsterTypeNone)
}

func (s SorceressLevelingLightning) KillIzual() error {
	m, _ := s.Data.Monsters.FindOne(npc.Izual, data.MonsterTypeUnique)
	_ = step.SecondaryAttack(skill.StaticField, m.UnitID, s.staticFieldCasts(), step.Distance(1, 5))

	return s.killMonster(npc.Izual, data.MonsterTypeUnique)
}

func (s SorceressLevelingLightning) KillDiablo() error {
	timeout := time.Second * 20
	startTime := time.Now()
	diabloFound := false

	for {
		if time.Since(startTime) > timeout && !diabloFound {
			s.Logger.Error("Diablo was not found, timeout reached")
			return nil
		}

		diablo, found := s.Data.Monsters.FindOne(npc.Diablo, data.MonsterTypeUnique)
		if !found || diablo.Stats[stat.Life] <= 0 {
			// Already dead
			if diabloFound {
				return nil
			}

			// Keep waiting...
			time.Sleep(200)
			continue
		}

		diabloFound = true
		s.Logger.Info("Diablo detected, attacking")

		_ = step.SecondaryAttack(skill.StaticField, diablo.UnitID, s.staticFieldCasts(), step.Distance(1, 5))

		return s.killMonster(npc.Diablo, data.MonsterTypeUnique)
	}
}

func (s SorceressLevelingLightning) KillPindle() error {
	return s.killMonster(npc.DefiledWarrior, data.MonsterTypeSuperUnique)
}

func (s SorceressLevelingLightning) KillNihlathak() error {
	return s.killMonster(npc.Nihlathak, data.MonsterTypeSuperUnique)
}

func (s SorceressLevelingLightning) KillAncients() error {
	for _, m := range s.Data.Monsters.Enemies(data.MonsterEliteFilter()) {
		m, _ := s.Data.Monsters.FindOne(m.Name, data.MonsterTypeSuperUnique)

		step.SecondaryAttack(skill.StaticField, m.UnitID, s.staticFieldCasts(), step.Distance(8, 10))

		step.MoveTo(data.Position{X: 10062, Y: 12639})

		s.killMonster(m.Name, data.MonsterTypeSuperUnique)
	}
	return nil
}

func (s SorceressLevelingLightning) KillBaal() error {
	m, _ := s.Data.Monsters.FindOne(npc.BaalCrab, data.MonsterTypeUnique)
	step.SecondaryAttack(skill.StaticField, m.UnitID, s.staticFieldCasts(), step.Distance(1, 4))

	return s.killMonster(npc.BaalCrab, data.MonsterTypeUnique)
}

package main

import (
	featskills "idx/internal/features/skills"
)

func buildSkillsDeps(d sharedDepsResult) *featskills.SkillsInstallService {
	installer := featskills.NewEmbedSkillsInstaller()
	return featskills.NewSkillsInstallService(installer, d.rawOutput)
}

package main

import (
	"io"

	featskills "idx/internal/features/skills"
)

func buildSkillsDeps(output io.Writer) *featskills.SkillsInstallService {
	installer := featskills.NewEmbedSkillsInstaller()
	return featskills.NewSkillsInstallService(installer, output)
}

package related

type candidateScore struct {
	absPath          string
	termScore        float64
	coReadScore      float64
	gitCoChangeScore float64
}

func (c *candidateScore) finalScore() float64 {
	return gitCoChangeWeight*c.gitCoChangeScore +
		coReadMatrixWeight*c.coReadScore +
		termOverlapWeight*c.termScore
}

func (c *candidateScore) reason() string {
	active := boolToInt(c.gitCoChangeScore > 0) + boolToInt(c.coReadScore > 0) + boolToInt(c.termScore > 0)
	if active > 1 {
		return ReasonBoth
	}
	return dominantReason(c)
}

func dominantReason(c *candidateScore) string {
	switch {
	case c.gitCoChangeScore > 0:
		return ReasonGit
	case c.coReadScore > 0:
		return ReasonCoRead
	default:
		return ReasonTermOverlap
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

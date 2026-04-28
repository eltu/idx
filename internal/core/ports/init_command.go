package ports

// InitCommandInterface define o contrato para inicializar um projeto.
type InitCommandInterface interface {
	// RunFromPath executa a inicialização de índice a partir de um diretório específico.
	RunFromPath(projectPath string) error
}

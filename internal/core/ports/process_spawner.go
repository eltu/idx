package ports

// ProcessSpawner encapsula a lógica de iniciar processos em background.
// Permite mockar em testes.
type ProcessSpawner interface {
	// SpawnWatchProcess inicia um processo watch em background para o diretório dado.
	// Retorna o PID do processo iniciado ou erro se não conseguir iniciar.
	// Implementações devem garantir que o processo fica desacoplado do pai.
	SpawnWatchProcess(projectPath string) (int, error)
}

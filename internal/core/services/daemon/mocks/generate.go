package mocks

//go:generate mockgen -source=../../../ports/daemon_repository.go -destination=mock_daemon_repository.go -package=mocks
//go:generate mockgen -source=../../../ports/project_tree.go -destination=mock_project_tree.go -package=mocks
//go:generate mockgen -source=../../../ports/text_output.go -destination=mock_text_output.go -package=mocks
//go:generate mockgen -source=../../../ports/init_command.go -destination=mock_init_command.go -package=mocks
//go:generate mockgen -source=../../../ports/process_spawner.go -destination=mock_process_spawner.go -package=mocks
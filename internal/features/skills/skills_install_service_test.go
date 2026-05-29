package skills_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"idx/internal/features/skills"
	"idx/internal/features/skills/mocks"
)

func newSkillsTestService(t *testing.T) (*mocks.MockSkillsInstaller, *bytes.Buffer, *skills.SkillsInstallService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	installer := mocks.NewMockSkillsInstaller(ctrl)
	out := &bytes.Buffer{}
	svc := skills.NewSkillsInstallService(installer, out)
	return installer, out, svc
}

func TestSkillsInstallService_Install_SupportedEditors(t *testing.T) {
	t.Parallel()

	for _, editor := range skills.SupportedEditors {
		editor := editor
		t.Run(editor, func(t *testing.T) {
			t.Parallel()

			// Arrange
			installer, _, svc := newSkillsTestService(t)
			installer.EXPECT().Install(editor).Return(nil)

			// Act & Assert
			require.NoError(t, svc.Install(editor))
		})
	}
}

func TestSkillsInstallService_Install_RejectsUnknownEditor(t *testing.T) {
	t.Parallel()

	// Arrange
	installer, _, svc := newSkillsTestService(t)
	installer.EXPECT().Install(gomock.Any()).Times(0)

	// Act
	err := svc.Install("vim")

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "vim")
}

func TestSkillsInstallService_Install_RejectsEmptyEditor(t *testing.T) {
	t.Parallel()

	// Arrange
	installer, _, svc := newSkillsTestService(t)
	installer.EXPECT().Install(gomock.Any()).Times(0)

	// Act & Assert
	require.Error(t, svc.Install(""))
}

func TestSkillsInstallService_Install_PropagatesInstallerError(t *testing.T) {
	t.Parallel()

	// Arrange
	installer, _, svc := newSkillsTestService(t)
	installErr := errors.New("disk full")
	installer.EXPECT().Install("claude").Return(installErr)

	// Act
	err := svc.Install("claude")

	// Assert
	require.ErrorIs(t, err, installErr)
}

func TestSkillsInstallService_Install_OutputContainsEditorName(t *testing.T) {
	t.Parallel()

	// Arrange
	installer, out, svc := newSkillsTestService(t)
	installer.EXPECT().Install("copilot").Return(nil)

	// Act
	require.NoError(t, svc.Install("copilot"))

	// Assert
	assert.Contains(t, out.String(), "copilot")
}

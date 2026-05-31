package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	search "idx/internal/features/search"
)

// ---- configureSearchFlags shorthands ----

func TestConfigureSearchFlags_ShorthandRegistration_AllPVariantFlags(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newSearchCommand()

	tests := []struct {
		flag      string
		shorthand string
	}{
		{"ext", "e"},
		{"path", "p"},
		{"limit", "n"},
		{"context", "c"},
		{"files-only", "l"},
		{"json", "j"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.flag, func(t *testing.T) {
			t.Parallel()

			// Assert
			f := cmd.Flags().ShorthandLookup(tc.shorthand)
			assert.NotNil(t, f, "expected -%s shorthand for --%s flag", tc.shorthand, tc.flag)
		})
	}
}

// ---- deprecated flag aliases are still registered ----

func TestConfigureSearchFlags_DeprecatedAliases_StillRegistered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newSearchCommand()

	deprecated := []string{"agent-compact", "json-pretty", "size", "from"}
	for _, name := range deprecated {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.NotNil(t, cmd.Flags().Lookup(name), "deprecated flag --%s must remain registered for backward compat", name)
		})
	}
}

func TestConfigureSearchFlags_NewAliasFlags_Registered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newSearchCommand()

	newFlags := []string{"compact", "pretty", "limit", "skip", "any", "relax", "hits", "count", "time"}
	for _, name := range newFlags {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.NotNil(t, cmd.Flags().Lookup(name), "expected --%s flag to be registered", name)
		})
	}
}

// ---- options() merging logic ----

func TestSearchOptions_CompactFlag_SetsAgentCompact(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := &searchCommandConfig{compact: true, format: search.OutputText, operator: search.OperatorAND}

	// Act
	opts := cfg.options()

	// Assert
	assert.True(t, opts.AgentCompact)
}

func TestSearchOptions_AgentCompactFlag_BackwardCompat_SetsAgentCompact(t *testing.T) {
	t.Parallel()

	cfg := &searchCommandConfig{agentCompact: true, format: search.OutputText, operator: search.OperatorAND}
	assert.True(t, cfg.options().AgentCompact)
}

func TestSearchOptions_BothCompactFlags_SetsAgentCompact(t *testing.T) {
	t.Parallel()

	cfg := &searchCommandConfig{agentCompact: true, compact: true, format: search.OutputText, operator: search.OperatorAND}
	assert.True(t, cfg.options().AgentCompact)
}

func TestSearchOptions_PrettyFlag_SetsPrettyJSON(t *testing.T) {
	t.Parallel()

	cfg := &searchCommandConfig{pretty: true, format: search.OutputText, operator: search.OperatorAND}
	assert.True(t, cfg.options().PrettyJSON)
}

func TestSearchOptions_HitsFlag_SetsMatchesOnly(t *testing.T) {
	t.Parallel()

	cfg := &searchCommandConfig{hitsOnly: true, format: search.OutputText, operator: search.OperatorAND}
	assert.True(t, cfg.options().MatchesOnly)
}

func TestSearchOptions_SkipAndFrom_AreAdditive(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := &searchCommandConfig{from: 5, skip: 3, format: search.OutputText, operator: search.OperatorAND}

	// Act + Assert
	assert.Equal(t, 8, cfg.options().From)
}

func TestSearchOptions_LimitIsUsed_WhenSizeNotSet(t *testing.T) {
	t.Parallel()

	cfg := &searchCommandConfig{limit: 10, size: 0, format: search.OutputText, operator: search.OperatorAND}
	assert.Equal(t, 10, cfg.options().Size)
}

func TestSearchOptions_CountOnly_SetsFilesOnly(t *testing.T) {
	t.Parallel()

	cfg := &searchCommandConfig{countOnly: true, format: search.OutputText, operator: search.OperatorAND}
	opts := cfg.options()
	assert.True(t, opts.FilesOnly)
	assert.True(t, opts.CountOnly)
}

func TestSearchOptions_TimingFlag_SetsTimingInOptions(t *testing.T) {
	t.Parallel()

	cfg := &searchCommandConfig{timing: true, format: search.OutputText, operator: search.OperatorAND}
	assert.True(t, cfg.options().Timing)
}

// ---- --any flag sets operator OR ----

func TestRunSearchCommand_AnyFlag_SetsOperatorOR(t *testing.T) {
	t.Parallel()

	// Arrange
	searcher := &stubSearcher{}
	runner := NewCommandRunner([]string{"idx", "search", "hello"}, nil, nil, searcher)
	cfg := &searchCommandConfig{
		format:   search.OutputText,
		operator: search.OperatorAND,
		anyMode:  true,
		// relaxIntSet=false: --relax was not explicitly passed, so no synthesis
	}
	cmd := runner.newSearchCommand()

	// Act
	err := runner.runSearchCommand(cmd, []string{"hello"}, cfg)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, search.OperatorOR, cfg.operator)
}

// ---- --relax N synthesizes '>N' format ----

func TestValidateSearchRelaxation_RelaxInt_SynthesizesGreaterThanFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		relaxInt int
		wantMin  int
	}{
		{"zero", 0, 0},
		{"one", 1, 1},
		{"five", 5, 5},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Arrange — relaxIntSet=true means the flag was explicitly provided
			cfg := &searchCommandConfig{
				operator:    search.OperatorAND,
				relaxInt:    tc.relaxInt,
				relaxIntSet: true,
			}

			// Act
			err := validateSearchRelaxation(cfg)

			// Assert
			require.NoError(t, err)
			assert.True(t, cfg.relaxationEnabled)
			assert.Equal(t, tc.wantMin, cfg.relaxationMin)
		})
	}
}

func TestValidateSearchRelaxation_RelaxIntNotSet_IsNoop(t *testing.T) {
	t.Parallel()

	// relaxIntSet=false means --relax was not passed; synthesize nothing
	cfg := &searchCommandConfig{
		operator:    search.OperatorAND,
		relaxInt:    0,
		relaxIntSet: false,
	}

	err := validateSearchRelaxation(cfg)

	require.NoError(t, err)
	assert.False(t, cfg.relaxationEnabled)
}

func TestValidateSearchRelaxation_RelaxIntWithOperatorOR_ReturnsError(t *testing.T) {
	t.Parallel()

	cfg := &searchCommandConfig{
		operator:    search.OperatorOR,
		relaxInt:    1,
		relaxIntSet: true,
	}
	require.Error(t, validateSearchRelaxation(cfg))
}

// ---- validateSearchFlagValues extended ----

func TestValidateSearchFlagValues_NegativeSkip_ReturnsError(t *testing.T) {
	t.Parallel()

	err := validateSearchFlagValues(0, -1, 0, 0, 0, false, false)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid --skip")
}

func TestValidateSearchFlagValues_ZeroLimitExplicitlySet_ReturnsError(t *testing.T) {
	t.Parallel()

	err := validateSearchFlagValues(0, 0, 0, 0, 0, false, true)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid --limit")
}

// ---- validateSearchFormat extended ----

func TestValidateSearchFormat_PrettyWithoutJSON_ReturnsError(t *testing.T) {
	t.Parallel()

	err := validateSearchFormat(search.OutputText, true)
	require.Error(t, err)
	assert.ErrorContains(t, err, "--pretty requires")
}

// ---- search command alias ----

func TestNewSearchCommand_FindAlias_Registered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newSearchCommand()
	assert.Contains(t, cmd.Aliases, "find")
}

// ---- Long description present ----

func TestNewSearchCommand_HasLongDescription(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newSearchCommand()
	assert.NotEmpty(t, cmd.Long)
}

// ---- --size deprecated flag forwarding ----

func TestSearchOptions_DeprecatedSizeFlag_ForwardedToLimit(t *testing.T) {
	t.Parallel()

	// Arrange — simulate user explicitly setting --size 5 when cfg.Size=20
	// runSearchCommand sets config.limit = config.size when --size changed
	cfg := &searchCommandConfig{
		limit:    5, // as if runSearchCommand copied size→limit
		size:     5,
		format:   search.OutputText,
		operator: search.OperatorAND,
	}

	// Act
	opts := cfg.options()

	// Assert — size 5 is preserved, not overridden by a higher limit default
	assert.Equal(t, 5, opts.Size)
}

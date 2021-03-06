package test

import (
	"testing"

	"github.com/golangci/golangci-lint/test/testshared"

	"github.com/golangci/golangci-lint/pkg/exitcodes"
)

func TestNoIssues(t *testing.T) {
	testshared.NewLintRunner(t).Run(getProjectRoot()).ExpectNoIssues()
}

func TestAutogeneratedNoIssues(t *testing.T) {
	testshared.NewLintRunner(t).Run(getTestDataDir("autogenerated")).ExpectNoIssues()
}

func TestEmptyDirRun(t *testing.T) {
	testshared.NewLintRunner(t).Run(getTestDataDir("nogofiles")).
		ExpectExitCode(exitcodes.NoGoFiles).
		ExpectOutputContains(": no go files to analyze")
}

func TestNotExistingDirRun(t *testing.T) {
	testshared.NewLintRunner(t).Run(getTestDataDir("no_such_dir")).
		ExpectHasIssue(`cannot find package \"./testdata/no_such_dir\"`)
}

func TestSymlinkLoop(t *testing.T) {
	testshared.NewLintRunner(t).Run(getTestDataDir("symlink_loop", "...")).ExpectNoIssues()
}

func TestDeadline(t *testing.T) {
	testshared.NewLintRunner(t).Run("--deadline=1ms", getProjectRoot()).
		ExpectExitCode(exitcodes.Timeout).
		ExpectOutputContains(`Deadline exceeded: try increase it by passing --deadline option`)
}

func TestTestsAreLintedByDefault(t *testing.T) {
	testshared.NewLintRunner(t).Run(getTestDataDir("withtests")).
		ExpectHasIssue("if block ends with a return")
}

func TestCgoOk(t *testing.T) {
	testshared.NewLintRunner(t).Run("--enable-all", getTestDataDir("cgo")).ExpectNoIssues()
}

func TestCgoWithIssues(t *testing.T) {
	testshared.NewLintRunner(t).Run("--enable-all", getTestDataDir("cgo_with_issues")).
		ExpectHasIssue("Printf format %t has arg cs of wrong type")
}

func TestUnsafeOk(t *testing.T) {
	testshared.NewLintRunner(t).Run("--enable-all", getTestDataDir("unsafe")).ExpectNoIssues()
}

func TestSkippedDirs(t *testing.T) {
	r := testshared.NewLintRunner(t).Run("--print-issued-lines=false", "--no-config", "--skip-dirs", "skip_me", "-Egolint",
		getTestDataDir("skipdirs", "..."))

	r.ExpectExitCode(exitcodes.IssuesFound).
		ExpectOutputEq("testdata/skipdirs/examples_no_skip/with_issue.go:8:9: if block ends with " +
			"a return statement, so drop this else and outdent its block (golint)\n")
}

func TestDeadcodeNoFalsePositivesInMainPkg(t *testing.T) {
	testshared.NewLintRunner(t).Run("--no-config", "--disable-all", "-Edeadcode", getTestDataDir("deadcode_main_pkg")).ExpectNoIssues()
}

func TestIdentifierUsedOnlyInTests(t *testing.T) {
	testshared.NewLintRunner(t).Run("--no-config", "--disable-all", "-Eunused", getTestDataDir("used_only_in_tests")).ExpectNoIssues()
}

func TestConfigFileIsDetected(t *testing.T) {
	checkGotConfig := func(r *testshared.RunResult) {
		r.ExpectExitCode(exitcodes.Success).
			ExpectOutputEq("test\n") // test config contains InternalTest: true, it triggers such output
	}

	r := testshared.NewLintRunner(t)
	checkGotConfig(r.Run(getTestDataDir("withconfig", "pkg")))
	checkGotConfig(r.Run(getTestDataDir("withconfig", "...")))
}

func TestEnableAllFastAndEnableCanCoexist(t *testing.T) {
	r := testshared.NewLintRunner(t)
	r.Run("--fast", "--enable-all", "--enable=typecheck").ExpectNoIssues()
	r.Run("--enable-all", "--enable=typecheck").ExpectExitCode(exitcodes.Failure)
}

func TestEnabledPresetsAreNotDuplicated(t *testing.T) {
	testshared.NewLintRunner(t).Run("--no-config", "-v", "-p", "style,bugs").
		ExpectOutputContains("Active presets: [bugs style]")
}

func TestDisallowedOptionsInConfig(t *testing.T) {
	type tc struct {
		cfg    string
		option string
	}

	cases := []tc{
		{
			cfg: `
				ruN:
					Args:
						- 1
			`,
		},
		{
			cfg: `
				run:
					CPUProfilePath: path
			`,
			option: "--cpu-profile-path=path",
		},
		{
			cfg: `
				run:
					MemProfilePath: path
			`,
			option: "--mem-profile-path=path",
		},
		{
			cfg: `
				run:
					Verbose: true
			`,
			option: "-v",
		},
	}

	r := testshared.NewLintRunner(t)
	for _, c := range cases {
		// Run with disallowed option set only in config
		r.RunWithYamlConfig(c.cfg).ExpectExitCode(exitcodes.Failure)

		if c.option == "" {
			continue
		}

		args := []string{c.option, "--fast"}

		// Run with disallowed option set only in command-line
		r.Run(args...).ExpectExitCode(exitcodes.Success)

		// Run with disallowed option set both in command-line and in config
		r.RunWithYamlConfig(c.cfg, args...).ExpectExitCode(exitcodes.Failure)
	}
}

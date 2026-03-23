package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/block/drift/compare"
	"github.com/block/drift/tui"
	goterm "golang.org/x/term"
)

// Version is set at build time via ldflags.
var Version = "dev"

type cmd struct {
	PathA   string           `arg:"" optional:"" help:"First path or git ref to compare."`
	PathB   string           `arg:"" optional:"" help:"Second path or git ref to compare."`
	Dir     string           `short:"C" help:"Change to this directory before doing anything." default:"" type:"existingdir"`
	Mode    string           `short:"m" help:"Force comparison mode (tree, binary, plist, text, image)." default:""`
	Git     bool             `help:"Force git mode (compare refs, not paths)." default:"false"`
	JSON    bool             `help:"Force JSON output."`
	Version kong.VersionFlag `help:"Print version and exit."`
}

func (c *cmd) run() error {
	if c.Dir != "" {
		if err := os.Chdir(c.Dir); err != nil {
			return fmt.Errorf("cannot change to directory %s: %w", c.Dir, err)
		}
	}

	interactive := !c.JSON && goterm.IsTerminal(int(os.Stdout.Fd()))

	result, err := c.resolve()
	if err != nil {
		return err
	}

	if interactive {
		return tui.Run(result)
	}

	// For standalone (non-tree) modes, automatically include the detail diff.
	if result.Mode != compare.ModeTree && result.Mode != compare.ModeGit {
		detail, err := compare.Detail(result, result.Root)
		if err != nil {
			return err
		}
		return outputJSON(standaloneOutput{
			Result: result,
			Detail: detail,
		})
	}

	return outputJSON(result)
}

// resolve determines the comparison mode and returns the result.
func (c *cmd) resolve() (*compare.Result, error) {
	// No args: git working tree mode.
	if c.PathA == "" && c.PathB == "" {
		if !compare.IsGitRepo() {
			return nil, fmt.Errorf("no arguments provided and not in a git repository\n\nUsage: drift <pathA> <pathB>")
		}
		return compare.CompareGitWorkTree()
	}

	// --git flag: treat args as git refs unconditionally.
	if c.Git {
		refA := c.PathA
		refB := c.PathB
		if refB == "" {
			// Single arg with --git: compare ref against HEAD.
			refB = refA
			refA = "HEAD"
		}
		return compare.Compare(refA, refB, compare.ModeGit)
	}

	// Single arg: try as git ref, compare against HEAD.
	if c.PathB == "" {
		if !compare.IsGitRepo() {
			return nil, fmt.Errorf("single argument requires a git repository\n\nUsage: drift <pathA> <pathB>")
		}
		if _, err := compare.ResolveGitRef(c.PathA); err != nil {
			return nil, fmt.Errorf("%s is not a valid path or git ref", c.PathA)
		}
		return compare.Compare("HEAD", c.PathA, compare.ModeGit)
	}

	// Two args: check if both are filesystem paths first.
	_, errA := os.Stat(c.PathA)
	_, errB := os.Stat(c.PathB)

	if errA == nil && errB == nil {
		// Both exist on disk: use existing path comparison.
		return compare.Compare(c.PathA, c.PathB, c.Mode)
	}

	// At least one doesn't exist as a path: try git refs.
	if compare.IsGitRepo() {
		_, gitErrA := compare.ResolveGitRef(c.PathA)
		_, gitErrB := compare.ResolveGitRef(c.PathB)
		if gitErrA == nil && gitErrB == nil {
			return compare.Compare(c.PathA, c.PathB, compare.ModeGit)
		}
	}

	// Fall through: report the original filesystem error.
	if errA != nil {
		return nil, fmt.Errorf("cannot access %s: %w", c.PathA, errA)
	}
	return nil, fmt.Errorf("cannot access %s: %w", c.PathB, errB)
}

type standaloneOutput struct {
	*compare.Result
	Detail *compare.DetailResult `json:"detail"`
}

func outputJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func main() {
	var cli cmd
	ctx := kong.Parse(
		&cli,
		kong.Name("drift"),
		kong.Description("Compare files, directories, archives, binaries, and git refs."),
		kong.UsageOnError(),
		kong.Vars{"version": Version},
	)
	ctx.FatalIfErrorf(cli.run())
}

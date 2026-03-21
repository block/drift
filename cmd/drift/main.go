package main

import (
	"encoding/json"
	"os"

	"github.com/alecthomas/kong"
	"github.com/block/drift/compare"
	"github.com/block/drift/tui"
	goterm "golang.org/x/term"
)

// Version is set at build time via ldflags.
var Version = "dev"

type cmd struct {
	PathA   string           `arg:"" help:"First path to compare."`
	PathB   string           `arg:"" help:"Second path to compare."`
	Mode    string           `short:"m" help:"Force comparison mode (tree, binary, plist, text)." default:""`
	JSON    bool             `help:"Force JSON output."`
	Version kong.VersionFlag `help:"Print version and exit."`
}

func (c *cmd) run() error {
	interactive := !c.JSON && goterm.IsTerminal(int(os.Stdout.Fd()))

	result, err := compare.Compare(c.PathA, c.PathB, c.Mode)
	if err != nil {
		return err
	}

	if interactive {
		return tui.Run(result)
	}

	// For standalone (non-tree) modes, automatically include the detail diff.
	if result.Mode != compare.ModeTree {
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
		kong.Description("Compare files, directories, archives, and binaries."),
		kong.UsageOnError(),
		kong.Vars{"version": Version},
	)
	ctx.FatalIfErrorf(cli.run())
}

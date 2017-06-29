package cmd

import (
	"context"
	"io"

	"github.com/pilosa/tools/bench"
	"github.com/spf13/cobra"
)

// NewBenchCommand subcommands
func NewZipfCommand(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	zipf := &bench.Zipf{}
	zipfCmd := &cobra.Command{
		Use:   "zipf",
		Short: "zipf sets random bits according to the Zipf distribution.",
		Long: `Sets random bits according to the Zipf distribution.

This is a power-law distribution controlled by two parameters.
Exponent, in the range (1, inf), with a default value of 1.001, controls
the "sharpness" of the distribution, with higher exponent being sharper.
Ratio, in the range (0, 1), with a default value of 0.25, controls the
maximum variation of the distribution, with higher ratio being more uniform.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()
			hosts, err := flags.GetStringSlice("hosts")
			if err != nil {
				return err
			}
			agentNum, err := flags.GetInt("agent-num")
			if err != nil {
				return err
			}
			result := bench.RunBenchmark(context.Background(), hosts, agentNum, zipf)
			err = PrintResults(cmd, result, stdout)
			if err != nil {
				return err
			}
			return nil
		},
	}

	flags := zipfCmd.Flags()
	flags.Int64Var(&zipf.BaseBitmapID, "base-bitmap-id", 0, "Rows being set will all be greater than this.")
	flags.Int64Var(&zipf.BitmapIDRange, "bitmap-id-range", 100000, "Number of possible row ids that can be set.")
	flags.Int64Var(&zipf.BaseProfileID, "base-profile-id", 0, "Column id to start from.")
	flags.Int64Var(&zipf.ProfileIDRange, "profile-id-range", 100000, "Number of possible column ids that can be set.")
	flags.IntVar(&zipf.Iterations, "iterations", 100, "Number of bits to set.")
	flags.Int64Var(&zipf.Seed, "seed", 1, "Seed for RNG.")
	flags.StringVar(&zipf.Frame, "frame", "zipf", "Pilosa frame in which to set bits.")
	flags.StringVar(&zipf.Index, "index", "benchindex", "Pilosa index to use.")
	flags.Float64Var(&zipf.BitmapExponent, "bitmap-exponent", 1.01, "Zipf exponent parameter for bitmap IDs.")
	flags.Float64Var(&zipf.BitmapRatio, "bitmap-ratio", 0.25, "Zipf probability ratio parameter for bitmap IDs.")
	flags.Float64Var(&zipf.ProfileExponent, "profile-exponent", 1.01, "Zipf exponent parameter for profile IDs.")
	flags.Float64Var(&zipf.ProfileRatio, "profile-ratio", 0.25, "Zipf probability ratio parameter for profile IDs.")
	flags.StringVar(&zipf.ClientType, "client-type", "single", "Can be 'single' (all agents hitting one host) or 'round_robin'.")
	flags.StringVar(&zipf.Operation, "operation", "set", "Can be set or clear.")
	flags.StringVar(&zipf.ContentType, "content-type", "protobuf", "Can be protobuf or pql.")

	return zipfCmd
}

func init() {
	benchCommandFns["zipf"] = NewZipfCommand
}

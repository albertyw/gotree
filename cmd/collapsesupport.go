package cmd

import (
	"github.com/spf13/cobra"
)

var lowSupportThreshold float64

// collapsesupportCmd represents the collapsesupport command
var collapsesupportCmd = &cobra.Command{
	Use:   "support",
	Short: "Collapse lowly supported branches of the input tree",
	Long: `Collapse lowly supported branches of the input tree.

Lowly supported branches are defined by a threshold (-s). All branches 
with support <= threshold are removed.
`,
	Run: func(cmd *cobra.Command, args []string) {
		for t := range collapseIntrees {
			t.Tree.CollapseLowSupport(lowSupportThreshold)
			collapseOutTrees.WriteString(t.Tree.Newick() + "\n")
		}
	},
}

func init() {
	collapseCmd.AddCommand(collapsesupportCmd)
	collapsesupportCmd.Flags().Float64VarP(&lowSupportThreshold, "support", "s", 0.0, "Support cutoff to collapse branches")
}
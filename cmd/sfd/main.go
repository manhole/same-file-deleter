package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"same-file-deleter/internal/app"
)

type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printRootUsage(os.Stderr)
		return 2
	}

	switch args[0] {
	case "index":
		return runIndex(args[1:])
	case "plan":
		return runPlan(args[1:])
	case "apply":
		return runApply(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", args[0])
		printRootUsage(os.Stderr)
		return 2
	}
}

func runIndex(args []string) int {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	dir := fs.String("dir", "", "target directory")
	out := fs.String("out", "", "output index jsonl path")
	update := fs.Bool("update", false, "reuse unchanged file checksums from existing index")
	var excludes multiStringFlag
	fs.Var(&excludes, "exclude", "exclude pattern (repeatable)")

	if err := fs.Parse(args); err != nil {
		printIndexUsage(os.Stderr)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "index does not accept positional arguments")
		printIndexUsage(os.Stderr)
		return 2
	}

	uc := app.IndexUseCase{
		Stderr: os.Stderr,
	}
	summary, err := uc.Run(app.IndexParams{
		Dir:      *dir,
		Out:      *out,
		Update:   *update,
		Excludes: excludes,
	})
	if err != nil {
		return reportError(err)
	}

	fmt.Fprintf(os.Stdout, "index complete: scanned=%d reused=%d rehashed=%d errors=%d\n",
		summary.Scanned, summary.Reused, summary.Rehashed, summary.Errors)

	if summary.Errors > 0 {
		return 1
	}
	return 0
}

func runPlan(args []string) int {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	a := fs.String("a", "", "A index jsonl path")
	b := fs.String("b", "", "B index jsonl path")
	out := fs.String("out", "", "output plan jsonl path")

	if err := fs.Parse(args); err != nil {
		printPlanUsage(os.Stderr)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "plan does not accept positional arguments")
		printPlanUsage(os.Stderr)
		return 2
	}

	uc := app.PlanUseCase{
		Stderr: os.Stderr,
	}
	summary, err := uc.Run(app.PlanParams{
		AIndexPath: *a,
		BIndexPath: *b,
		Out:        *out,
	})
	if err != nil {
		return reportError(err)
	}

	fmt.Fprintf(os.Stdout, "plan complete: a_records=%d b_records=%d matches=%d match_bytes=%d\n",
		summary.ARecords, summary.BRecords, summary.Matches, summary.MatchBytes)
	return 0
}

func runApply(args []string) int {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	plan := fs.String("plan", "", "plan jsonl path")
	execute := fs.Bool("execute", false, "execute deletions (default: dry-run)")
	maxDelete := fs.Int("max-delete", 0, "abort if plan candidate count exceeds this value")

	if err := fs.Parse(args); err != nil {
		printApplyUsage(os.Stderr)
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "apply does not accept positional arguments")
		printApplyUsage(os.Stderr)
		return 2
	}

	uc := app.ApplyUseCase{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	summary, err := uc.Run(app.ApplyParams{
		PlanPath:  *plan,
		Execute:   *execute,
		MaxDelete: *maxDelete,
	})
	if err != nil {
		return reportError(err)
	}

	fmt.Fprintf(os.Stdout, "apply complete: candidates=%d deleted=%d failed=%d deleted_bytes=%d mode=%s\n",
		summary.Candidates,
		summary.Deleted,
		summary.Failed,
		summary.DeletedBytes,
		modeName(*execute),
	)
	if summary.Failed > 0 {
		return 1
	}
	return 0
}

func modeName(execute bool) string {
	if execute {
		return "execute"
	}
	return "dry-run"
}

func reportError(err error) int {
	if app.IsInputError(err) {
		fmt.Fprintf(os.Stderr, "input error: %v\n", err)
		return 2
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	return 1
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: sfd <index|plan|apply> [options]")
}

func printIndexUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: sfd index --dir <directory> --out <checksums.jsonl> [--update] [--exclude <glob> ...]")
}

func printPlanUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: sfd plan --a <A.checksums.jsonl> --b <B.checksums.jsonl> --out <delete-plan.jsonl>")
}

func printApplyUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: sfd apply --plan <delete-plan.jsonl> [--execute] [--max-delete <n>]")
}

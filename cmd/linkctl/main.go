// Command linkctl is the Linkforge operator CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/al-bo/rd-linkforge/internal/config"
)

const version = "dev"

type EnvLoader = func(string) (string, bool)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func loadConfig(ctx context.Context, env EnvLoader, logger *slog.Logger) {
	cl := config.NewEnvLoader(env)
	config, err := cl.Load(ctx)
	if err != nil {
		logger.Error("error loading config", slog.Any("error", err))
	} else {
		logger.Log(ctx, slog.LevelInfo, "configuration loaded", slog.Any("config", config))
	}
}

func run(ctx context.Context, args []string, env EnvLoader, stdout, stderr io.Writer) int {
	h := slog.NewTextHandler(stderr, nil)
	logger := slog.New(h)

	exportCmd := flag.NewFlagSet("export", flag.ContinueOnError)
	dbPath := exportCmd.String("db", "", "Path to database (required)")
	outPath := exportCmd.String("out", "", "Path to output (optional)")
	exportCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: linkctl export -db FILE [-out FILE]\n\n flags:\n")
		exportCmd.PrintDefaults()
	}

	importCmd := flag.NewFlagSet("import", flag.ContinueOnError)
	inPath := importCmd.String("in", "", "path to source file (required)")
	dstPath := importCmd.String("db", "", "Path to database (required)")
	importCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: linkctl import -in File -db FILE\n\n flags:\n")
		importCmd.PrintDefaults()
	}

	if len(args) == 0 {
		fmt.Println("Wrong command")
		return 2
	}
	switch args[0] {
	case "import":
		{
			fmt.Println("Import")
			importCmd.Parse(args[1:])
			if *inPath != "" && *dstPath != "" {
				fmt.Println("Success start import")
			} else {
				fmt.Println("Wrong import flags")
				return 1
			}
			break
		}
	case "config":
		{
			loadConfig(ctx, env, logger)
			break
		}
	case "export":
		{
			exportCmd.Parse(args[1:])
			if *dbPath == "" {
				fmt.Fprintln(os.Stderr, "Error: flag -db is required.")
				exportCmd.Usage()
				return 1
			}

			fmt.Printf("export database: %s\n", *dbPath)
			if *outPath != "" {
				fmt.Printf("Export result to file: %s\n", *outPath)
			} else {
				fmt.Println("Export database to stdout")
			}

			return 0
		}
	default:
		{
			fmt.Println("Wrong command")
			return 2
		}
	}

	return 0
}

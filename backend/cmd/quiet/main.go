// Command quiet is the Quiet backend: API server with the daily scheduler,
// plus terminal subcommands for running topics by hand.
//
//	quiet serve                     API on 127.0.0.1:8080 + daily job at 05:30
//	quiet run [-follow] "<text>"    turn text into a topic, run it once, print the result
//	quiet daily                     run the daily job once now
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/robfig/cron/v3"

	"quiet/internal/api"
	"quiet/internal/db"
	"quiet/internal/job"
	"quiet/internal/llm"
	"quiet/internal/search"
)

const listenAddr = "127.0.0.1:8080"

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	if len(os.Args) < 2 {
		usage()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(ctx)
	case "run":
		err = runOnce(ctx, os.Args[2:])
	case "daily":
		err = daily(ctx)
	default:
		usage()
	}
	if err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: quiet serve | quiet run [-follow] "<topic text>" | quiet daily`)
	os.Exit(2)
}

type config struct {
	tavilyKey, anthropicKey, model, appToken, dbPath string
	trialCap                                         int
	loc                                              *time.Location
}

func loadConfig(needToken bool) (config, error) {
	c := config{
		tavilyKey:    os.Getenv("TAVILY_API_KEY"),
		anthropicKey: os.Getenv("ANTHROPIC_API_KEY"),
		model:        envOr("MODEL", "claude-haiku-5-5"),
		appToken:     os.Getenv("APP_TOKEN"),
		dbPath:       envOr("DB_PATH", "quiet.db"),
	}
	var missing []string
	if c.tavilyKey == "" {
		missing = append(missing, "TAVILY_API_KEY")
	}
	if c.anthropicKey == "" {
		missing = append(missing, "ANTHROPIC_API_KEY")
	}
	if needToken && len(c.appToken) < 16 {
		missing = append(missing, "APP_TOKEN (at least 16 characters)")
	}
	if len(missing) > 0 {
		return c, fmt.Errorf("missing config: %v", missing)
	}
	var err error
	if c.trialCap, err = strconv.Atoi(envOr("TRIAL_CAP_PER_DAY", "10")); err != nil {
		return c, fmt.Errorf("TRIAL_CAP_PER_DAY: %w", err)
	}
	if c.loc, err = time.LoadLocation(envOr("TZ", "Asia/Saigon")); err != nil {
		return c, fmt.Errorf("TZ: %w", err)
	}
	return c, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func setup(ctx context.Context, needToken bool) (config, *job.Runner, error) {
	c, err := loadConfig(needToken)
	if err != nil {
		return c, nil, err
	}
	store, err := db.Open(ctx, c.dbPath)
	if err != nil {
		return c, nil, err
	}
	runner := &job.Runner{
		DB:       store,
		Searcher: &search.Tavily{APIKey: c.tavilyKey},
		LLM:      &llm.Anthropic{APIKey: c.anthropicKey, Model: c.model},
		Loc:      c.loc,
	}
	return c, runner, nil
}

func serve(ctx context.Context) error {
	c, runner, err := setup(ctx, true)
	if err != nil {
		return err
	}
	defer runner.DB.Close()

	sched := cron.New(cron.WithLocation(c.loc))
	if _, err := sched.AddFunc("30 5 * * *", func() {
		if _, err := runner.RunDaily(ctx); err != nil {
			log.Printf("daily: %v", err)
		}
	}); err != nil {
		return err
	}
	sched.Start()
	defer sched.Stop()

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           (&api.Server{DB: runner.DB, Runner: runner, Token: c.appToken, TrialCapPerDay: c.trialCap}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      15 * time.Minute, // run-daily is synchronous
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Printf("quiet: listening on %s, daily job at 05:30 %s, model %s", listenAddr, c.loc, c.model)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func runOnce(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	follow := fs.Bool("follow", false, "follow the topic daily (use this to seed topics)")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
	}
	_, runner, err := setup(ctx, false)
	if err != nil {
		return err
	}
	defer runner.DB.Close()

	t, err := api.CreateTrial(ctx, runner, fs.Arg(0))
	if t.ID == 0 {
		return err
	}
	fmt.Printf("%s  (topic %d)\n%s\nquery: %s\n\n", t.Name, t.ID, t.Understood, t.Query)
	if err != nil {
		return err
	}
	if *follow {
		if err := runner.DB.SetStatus(ctx, t.ID, db.StatusFollowing); err != nil {
			return err
		}
		fmt.Println("following daily")
	}
	items, err := runner.DB.ItemsForRunDate(ctx, t.ID, runner.Today())
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("(nothing worth keeping today)")
	}
	for i, it := range items {
		fmt.Printf("%d. %s\n   %s\n", i+1, it.Title, it.URL)
		if it.Summary != nil {
			fmt.Printf("   %s\n", *it.Summary)
		} else {
			fmt.Println("   (no summary: snippet too thin)")
		}
		fmt.Println()
	}
	return nil
}

func daily(ctx context.Context) error {
	_, runner, err := setup(ctx, false)
	if err != nil {
		return err
	}
	defer runner.DB.Close()
	rep, err := runner.RunDaily(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%d topics, %d ok, %d items kept, failed: %v\n", rep.Topics, rep.OK, rep.Kept, rep.Failed)
	return nil
}

// cmd/schemadiff crawls two F5OS devices (different versions) via RESTCONF,
// compares their YANG module lists and API response structures, then writes a
// Markdown report highlighting breaking changes, new APIs, and new properties.
//
// Usage:
//
//	export SCHEMA_BASE_PASS=admin
//	export SCHEMA_NEW_PASS=admin
//	schemadiff \
//	  -base-host 10.0.0.1:8888 -base-user admin \
//	  -new-host 10.0.0.2:8888  -new-user admin  \
//	  -out report.md
//
// Passwords must be provided via SCHEMA_BASE_PASS and SCHEMA_NEW_PASS
// environment variables. CLI flags for passwords are intentionally not
// supported to avoid leaking credentials in the process table and shell
// history.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	// ── flags ──────────────────────────────────────────────────────────
	baseHost := flag.String("base-host", envOr("SCHEMA_BASE_HOST", ""), "Base F5OS host:port")
	baseUser := flag.String("base-user", envOr("SCHEMA_BASE_USER", "admin"), "Base F5OS username")

	newHost := flag.String("new-host", envOr("SCHEMA_NEW_HOST", ""), "New F5OS host:port")
	newUser := flag.String("new-user", envOr("SCHEMA_NEW_USER", "admin"), "New F5OS username")

	outFile := flag.String("out", envOr("SCHEMA_DIFF_REPORT", "schema_diff_report.md"), "Output report path")
	flag.Parse()

	// Passwords are read from environment variables only — never from CLI
	// flags — so they don't appear in the process table or shell history.
	basePass := os.Getenv("SCHEMA_BASE_PASS")
	newPass := os.Getenv("SCHEMA_NEW_PASS")

	if *baseHost == "" || *newHost == "" {
		fmt.Fprintln(os.Stderr, "error: -base-host and -new-host (or SCHEMA_BASE_HOST/SCHEMA_NEW_HOST) are required")
		flag.Usage()
		os.Exit(1)
	}
	if basePass == "" || newPass == "" {
		fmt.Fprintln(os.Stderr, "error: SCHEMA_BASE_PASS and SCHEMA_NEW_PASS environment variables are required")
		os.Exit(1)
	}

	// ── connect to both devices ────────────────────────────────────────
	log.Println("Connecting to base device:", *baseHost)
	baseClient, err := newF5osClient(*baseHost, *baseUser, basePass)
	if err != nil {
		log.Fatalf("base device connection failed: %v", err)
	}
	log.Printf("Base device: platform=%s version=%s\n", baseClient.platformType, baseClient.platformVersion)

	log.Println("Connecting to new device:", *newHost)
	newClient, err := newF5osClient(*newHost, *newUser, newPass)
	if err != nil {
		log.Fatalf("new device connection failed: %v", err)
	}
	log.Printf("New device: platform=%s version=%s\n", newClient.platformType, newClient.platformVersion)

	// ── fetch YANG library modules from both devices ───────────────────
	log.Println("Fetching YANG library from base device...")
	baseModules, err := baseClient.fetchYANGModules()
	if err != nil {
		log.Printf("warning: YANG library fetch failed on base (%v), falling back to API-only comparison", err)
	}
	log.Printf("Base device: %d YANG modules\n", len(baseModules))

	log.Println("Fetching YANG library from new device...")
	newModules, err := newClient.fetchYANGModules()
	if err != nil {
		log.Printf("warning: YANG library fetch failed on new (%v), falling back to API-only comparison", err)
	}
	log.Printf("New device: %d YANG modules\n", len(newModules))

	// ── crawl API endpoints ────────────────────────────────────────────
	log.Println("Crawling API endpoints on base device...")
	baseAPIs := baseClient.crawlEndpoints()
	log.Printf("Base device: %d endpoints crawled\n", len(baseAPIs))

	log.Println("Crawling API endpoints on new device...")
	newAPIs := newClient.crawlEndpoints()
	log.Printf("New device: %d endpoints crawled\n", len(newAPIs))

	// ── diff ───────────────────────────────────────────────────────────
	log.Println("Computing diff...")
	diff := computeDiff(baseModules, newModules, baseAPIs, newAPIs)

	// ── report ─────────────────────────────────────────────────────────
	report := generateReport(
		baseClient.platformType, baseClient.platformVersion,
		newClient.platformType, newClient.platformVersion,
		diff,
	)

	if err := os.WriteFile(*outFile, []byte(report), 0644); err != nil {
		log.Fatalf("failed to write report: %v", err)
	}
	log.Printf("Report written to %s\n", *outFile)

	// exit 1 if there are breaking changes so CI can flag it
	if len(diff.BreakingChanges) > 0 {
		log.Printf("WARNING: %d breaking changes detected\n", len(diff.BreakingChanges))
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

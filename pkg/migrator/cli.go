package migrator

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// RunCLI executes the migration with rich verbose command line logs (non-TUI).
func RunCLI(ctx context.Context, cfg Config) (*Summary, error) {
	eventCh := make(chan ProgressEvent, 100)
	confirmCh := make(chan bool, 1)

	var bgSummary *Summary
	var bgErr error
	go func() {
		bgSummary, bgErr = Run(ctx, cfg, eventCh, confirmCh)
		close(eventCh)
	}()

	for e := range eventCh {
		timeStr := time.Now().Format("15:04:05")
		switch e.Type {
		case EventScanStart:
			fmt.Printf("[%s] 🔍 %s\n", timeStr, e.Message)
		case EventScanProgress:
			if e.Current <= 10 || e.Current%50 == 0 {
				fmt.Printf("[%s] 📂 Discovered %d video files... (%s)\n", timeStr, e.Current, e.Detail)
			}
		case EventScanDone:
			fmt.Printf("[%s] ✨ %s\n", timeStr, e.Message)
		case EventPlanItem:
			if e.Current <= 10 || e.Current%25 == 0 || e.Current == e.Total {
				fmt.Printf("[%s] [%d/%d] [PLAN] %s -> %s\n", timeStr, e.Current, e.Total, e.MovieID, e.TargetPath)
			}
		case EventConfirmReady:
			fmt.Printf("\n[%s] 📋 %s\n", timeStr, e.Message)
			fmt.Print("👉 Proceed with live migration? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			ans, _ := reader.ReadString('\n')
			ans = strings.TrimSpace(strings.ToLower(ans))
			if ans == "y" || ans == "yes" {
				confirmCh <- true
			} else {
				confirmCh <- false
			}
		case EventMoveStart:
			if e.Current%25 == 0 || e.Current == e.Total {
				fmt.Printf("[%s] [%d/%d] 🚚 Moving %s -> %s\n", timeStr, e.Current, e.Total, e.MovieID, e.Actress)
			}
		case EventMoveDone:
			if e.Current <= 5 || e.Current%25 == 0 || e.Current == e.Total {
				fmt.Printf("[%s] [%d/%d] ✅ Organized %s -> %s\n", timeStr, e.Current, e.Total, e.MovieID, e.TargetPath)
			}
		case EventMoveDuplicate:
			fmt.Printf("[%s] ℹ️  %s\n", timeStr, e.Message)
		case EventMoveSkip:
			fmt.Printf("[%s] ⚠️  %s\n", timeStr, e.Message)
		case EventMoveError:
			fmt.Printf("[%s] ❌ %s\n", timeStr, e.Message)
		case EventUpdateHTML:
			fmt.Printf("[%s] 🔄 %s\n", timeStr, e.Message)
		case EventCleanArchive:
			fmt.Printf("[%s] 🧹 %s\n", timeStr, e.Message)
		case EventDone:
			fmt.Printf("[%s] ✨ %s\n", timeStr, e.Message)
		}
	}

	return bgSummary, bgErr
}
